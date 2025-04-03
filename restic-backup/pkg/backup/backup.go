package backup

import (
	"log"
	"time"

	"github.com/yourusername/restic-backup/pkg/k8s"
	"github.com/yourusername/restic-backup/pkg/manifests"
)

// RunBackup executes the backup workflow.
func RunBackup(namespace, pvcName, vsc, awsID, awsSecret, repository, password string, repoInitialized bool) {
	vsName := pvcName + "-vs"
	clonePVCName := pvcName + "-clone"
	vsCreated := false
	pvcCloneCreated := false

	// Step 1: Create VolumeSnapshot.
	vsRepls := map[string]string{
		"PVC_NAME":                  pvcName,
		"VOLUME_SNAPSHOT_NAME":      vsName,
		"NAMESPACE":                 namespace,
		"VOLUME_SNAPSHOT_CLASSNAME": vsc,
	}
	vsManifest := k8s.ReplacePlaceholders(manifests.VolumeSnapshot, vsRepls)
	log.Printf("🔧 Creating VolumeSnapshot %s for PVC %s...", vsName, pvcName)
	if err := k8s.ApplyManifest(vsManifest, namespace, "", false); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to create VolumeSnapshot: %v", err)
	}
	vsCreated = true

	log.Printf("⌛ Waiting for VolumeSnapshot %s to be ready...", vsName)
	if err := k8s.WaitForVolumeSnapshot(vsName, namespace, 300*time.Second); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ VolumeSnapshot %s not ready: %v", vsName, err)
	}

	// Step 2: Create clone PVC.
	cloneRepls := map[string]string{
		"NEW_PVC_NAME":         clonePVCName,
		"VOLUME_SNAPSHOT_NAME": vsName,
		"NAMESPACE":            namespace,
	}
	sc, err := k8s.GetPVCStorageClass(pvcName, namespace)
	if err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to get storage class: %v", err)
	}
	ssize, err := k8s.GetPVCStorageSize(pvcName, namespace)
	if err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to get storage size: %v", err)
	}
	vmode, err := k8s.GetPVCVolumeMode(pvcName, namespace)
	if err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to get volume mode: %v", err)
	}
	cloneRepls["STORAGE_CLASS"] = sc
	cloneRepls["STORAGE_SIZE"] = ssize
	cloneRepls["VOLUME_MODE"] = vmode
	pvcCloneManifest := k8s.ReplacePlaceholders(manifests.PVCClone, cloneRepls)
	log.Printf("🔧 Creating PVC clone %s from VolumeSnapshot %s...", clonePVCName, vsName)
	if err := k8s.ApplyManifest(pvcCloneManifest, namespace, "", false); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to create PVC clone: %v", err)
	}
	pvcCloneCreated = true

	log.Printf("⌛ Waiting for PVC clone %s to become Bound...", clonePVCName)
	if err := k8s.WaitForPVCBound(clonePVCName, namespace, 300*time.Second); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ PVC clone did not become Bound: %v", err)
	}

	// Step 3: Retrieve source PVC's PV name.
	pvName, err := k8s.GetPVCVolumeName(pvcName, namespace)
	if err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to get PV name: %v", err)
	}

	// Step 4: Initialize repository if needed.
	if !repoInitialized {
		log.Println("🔧 Restic repository not initialized. Applying init job...")
		initRepls := map[string]string{
			"AWS_ACCESS_KEY_ID":     awsID,
			"AWS_SECRET_ACCESS_KEY": awsSecret,
			"RESTIC_REPOSITORY":     repository,
			"RESTIC_PASSWORD":       password,
			"NAMESPACE":             namespace,
		}
		initManifest := k8s.ReplacePlaceholders(manifests.ResticInitJob, initRepls)
		if err := k8s.ApplyManifest(initManifest, namespace, "", true); err != nil {
			k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
			log.Fatalf("❌ Failed to apply init job: %v", err)
		}
		if err := k8s.WaitForJob("restic-init", namespace, 30*time.Second); err != nil {
			k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
			log.Fatalf("❌ Init job did not complete: %v", err)
		}
	} else {
		log.Println("✅ Restic repository already initialized.")
	}

	// Step 5: Run backup job.
	backupRepls := map[string]string{
		"PVC_NAME":              clonePVCName,
		"NAMESPACE":             namespace,
		"PV_NAME":               pvName,
		"AWS_ACCESS_KEY_ID":     awsID,
		"AWS_SECRET_ACCESS_KEY": awsSecret,
		"RESTIC_REPOSITORY":     repository,
		"RESTIC_PASSWORD":       password,
	}
	backupManifest := k8s.ReplacePlaceholders(manifests.BackupJob, backupRepls)
	log.Println("🔧 Applying backup job manifest...")
	if err := k8s.ApplyManifest(backupManifest, namespace, clonePVCName, true); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to apply backup job manifest: %v", err)
	}

	// Launch log streaming to capture backup progress from the "backup" container.
	go func() {
		if err := k8s.StreamJobProgressPercentage("block-backup-job", namespace, "backup", "READ progress:"); err != nil {
			log.Printf("❌ Error streaming backup progress logs: %v", err)
		}
	}()

	log.Println("⌛ Waiting for backup job to complete...")
	if err := k8s.WaitForJob("block-backup-job", namespace, 3600*time.Second); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Backup job did not complete: %v", err)
	}
	log.Println("✅ Backup completed successfully.")
	k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
}
