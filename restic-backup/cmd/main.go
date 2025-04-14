package main

import (
	"flag"
	"log"
	"time"

	"github.com/yourusername/restic-backup/pkg/backup"
	"github.com/yourusername/restic-backup/pkg/find"
	"github.com/yourusername/restic-backup/pkg/k8s"
	"github.com/yourusername/restic-backup/pkg/manifests"
	"github.com/yourusername/restic-backup/pkg/restore"
)

func main() {
	// Define command-line flags.
	mode := flag.String("mode", "", "Operation mode: backup, restore, or find")
	pvcName := flag.String("pvc", "", "Name of the PVC to backup or the destination PVC for restore")
	namespace := flag.String("namespace", "backup", "Kubernetes namespace (default: backup)")
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file (optional)")
	vsc := flag.String("vsc", "my-volumesnapshotclass", "VolumeSnapshotClass name to use")
	awsID := flag.String("awsid", "", "AWS_ACCESS_KEY_ID for restic")
	awsSecret := flag.String("awssecret", "", "AWS_SECRET_ACCESS_KEY for restic")
	repository := flag.String("repository", "", "RESTIC_REPOSITORY value")
	password := flag.String("password", "", "RESTIC_PASSWORD value")
	// For restore mode:
	sourcePV := flag.String("sourcepv", "", "Original source PV name used for the backup (required in restore mode)")
	// For backup and find modes, the snapshot tag is required.
	snapshot := flag.String("snapshot", "", "Tag value for snapshot name (used by backup and find subcommands)")

	flag.Parse()

	// Combined flag checks.
	if *mode != "backup" && *mode != "restore" && *mode != "find" {
		log.Fatal("❌ Please specify -mode=backup, -mode=restore, or -mode=find")
	}
	if *namespace == "" {
		log.Fatal("❌ Please provide a valid namespace using -namespace")
	}
	if *awsID == "" || *awsSecret == "" || *repository == "" || *password == "" {
		log.Fatal("❌ Please provide all secret parameters: -awsid, -awssecret, -repository, -password")
	}

	// Mode-specific checks.
	switch *mode {
	case "backup":
		if *pvcName == "" {
			log.Fatal("❌ For backup mode, please provide the PVC name using -pvc")
		}
		if *snapshot == "" {
			log.Fatal("❌ For backup mode, please provide a snapshot tag value using -snapshot")
		}
	case "restore":
		if *pvcName == "" {
			log.Fatal("❌ For restore mode, please provide the destination PVC name using -pvc")
		}
		if *sourcePV == "" {
			log.Fatal("❌ For restore mode, please provide the source PV name using -sourcepv")
		}
	case "find":
		if *snapshot == "" {
			log.Fatal("❌ For find mode, please provide a snapshot tag value using -snapshot")
		}
	}

	// Initialize Kubernetes clients.
	if err := k8s.InitK8sClients(*kubeconfig); err != nil {
		log.Fatalf("❌ Error initializing Kubernetes clients: %v", err)
	}

	// Run repository check job for all modes.
	log.Println("🔧 Applying repository check job manifest...")
	jobSuffix, err := k8s.GenerateJobSuffix()
	if err != nil {
		log.Fatalf("❌ Failed to generate job suffix: %v", err)
	}
	checkRepls := map[string]string{
		"AWS_ACCESS_KEY_ID":     *awsID,
		"AWS_SECRET_ACCESS_KEY": *awsSecret,
		"RESTIC_REPOSITORY":     *repository,
		"RESTIC_PASSWORD":       *password,
	}
	checkJobName := "restic-check-" + jobSuffix
	if err := k8s.ApplyManifest(manifests.ResticCheckJob, *namespace, checkJobName, checkRepls); err != nil {
		log.Fatalf("❌ Failed to apply repository check job manifest: %v", err)
	}
	log.Println("⌛ Waiting for repository check job to complete...")
	checkErr := k8s.WaitForJob(checkJobName, *namespace, 10*time.Second)
	repoInitialized := (checkErr == nil)

	// For restore and find modes, repository must be initialized.
	if (*mode == "restore" || *mode == "find") && !repoInitialized {
		log.Fatal("❌ Repository is not initialized; cannot run restore or find subcommand")
	}

	switch *mode {
	case "backup":
		// Note: Update your backup.RunBackup in pkg/backup to accept the snapshot parameter.
		backup.RunBackup(*namespace, *pvcName, *snapshot, *vsc, *awsID, *awsSecret, *repository, *password, repoInitialized)
	case "restore":
		restore.RunRestore(*namespace, *pvcName, *sourcePV, *awsID, *awsSecret, *repository, *password)
	case "find":
		snapshotID, err := find.RunFind(*namespace, *snapshot, *awsID, *awsSecret, *repository, *password)
		if err != nil {
			log.Fatalf("❌ Find job failed: %v", err)
		}
		if snapshotID != "" {
			log.Printf("✅ Snapshot found with ID: %s", snapshotID)
		} else {
			log.Println("❌ Snapshot not found.")
		}
	}
}
