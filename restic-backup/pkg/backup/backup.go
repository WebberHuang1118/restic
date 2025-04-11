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
	// The VolumeSnapshot manifest now uses:
	//   name: {{NAME}}
	//   namespace: {{NAMESPACE}}
	// And expects the extra tokens "PVC_NAME" (the source PVC) and "VOLUME_SNAPSHOT_CLASSNAME" (the snapshot class).
	vsRepls := map[string]string{
		"PVC_NAME":                  pvcName,
		"VOLUME_SNAPSHOT_CLASSNAME": vsc,
	}
	if err := k8s.ApplyManifest(manifests.VolumeSnapshot, namespace, vsName, vsRepls); err != nil {
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
	// The PVCClone manifest uses:
	//   name: {{NAME}}
	//   namespace: {{NAMESPACE}}
	// It expects extra tokens "VOLUME_MODE", "STORAGE_CLASS", "STORAGE_SIZE", and "VOLUME_SNAPSHOT_NAME".
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
	cloneRepls := map[string]string{
		"VOLUME_MODE":          vmode,
		"STORAGE_CLASS":        sc,
		"STORAGE_SIZE":         ssize,
		"VOLUME_SNAPSHOT_NAME": vsName, // Used for dataSource.name in the PVC
	}
	if err := k8s.ApplyManifest(manifests.PVCClone, namespace, clonePVCName, cloneRepls); err != nil {
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
		jobSuffix, err := k8s.GenerateJobSuffix()
		if err != nil {
			log.Fatalf("❌ Failed to generate job suffix for init job: %v", err)
		}
		// For the init job, the manifest template uses tokens {{NAMESPACE}} and {{NAME}}.
		// We pass the default name as "restic-init-" + jobSuffix.
		initRepls := map[string]string{
			"AWS_ACCESS_KEY_ID":     awsID,
			"AWS_SECRET_ACCESS_KEY": awsSecret,
			"RESTIC_REPOSITORY":     repository,
			"RESTIC_PASSWORD":       password,
		}
		if err := k8s.ApplyManifest(manifests.ResticInitJob, namespace, "restic-init-"+jobSuffix, initRepls); err != nil {
			k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
			log.Fatalf("❌ Failed to apply init job: %v", err)
		}
		if err := k8s.WaitForJob("restic-init-"+jobSuffix, namespace, 30*time.Second); err != nil {
			k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
			log.Fatalf("❌ Init job did not complete: %v", err)
		}
	} else {
		log.Println("✅ Restic repository already initialized.")
	}

	// Step 5: Run backup job.
	jobSuffix, err := k8s.GenerateJobSuffix()
	if err != nil {
		log.Fatalf("❌ Failed to generate job suffix for backup job: %v", err)
	}
	backupRepls := map[string]string{
		"AWS_ACCESS_KEY_ID":     awsID,
		"AWS_SECRET_ACCESS_KEY": awsSecret,
		"RESTIC_REPOSITORY":     repository,
		"RESTIC_PASSWORD":       password,
		"PVC_NAME":              clonePVCName, // If the accelerated_io command expects a PVC name
		"PV_NAME":               pvName,
	}
	if err := k8s.ApplyManifest(manifests.BackupJob, namespace, "block-backup-job-"+jobSuffix, backupRepls); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Failed to apply backup job manifest: %v", err)
	}

	// Launch log streaming to capture backup progress.
	go func() {
		if err := k8s.StreamJobProgressPercentage("block-backup-job-"+jobSuffix, namespace, "backup", "READ progress:"); err != nil {
			log.Printf("❌ Error streaming backup progress logs: %v", err)
		}
	}()

	log.Println("⌛ Waiting for backup job to complete...")
	if err := k8s.WaitForJob("block-backup-job-"+jobSuffix, namespace, 3600*time.Second); err != nil {
		k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
		log.Fatalf("❌ Backup job did not complete: %v", err)
	}
	log.Println("✅ Backup completed successfully.")
	k8s.CleanupResources(namespace, vsName, clonePVCName, vsCreated, pvcCloneCreated)
}
