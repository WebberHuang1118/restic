package main

import (
	"flag"
	"log"
	"time"

	"github.com/yourusername/restic-backup/pkg/backup"
	"github.com/yourusername/restic-backup/pkg/k8s"
	"github.com/yourusername/restic-backup/pkg/manifests"
	"github.com/yourusername/restic-backup/pkg/restore"
)

func main() {
	mode := flag.String("mode", "", "Operation mode: backup or restore")
	pvcName := flag.String("pvc", "", "Name of the PVC to backup or the destination PVC for restore")
	namespace := flag.String("namespace", "backup", "Kubernetes namespace (default: backup)")
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file (optional)")
	vsc := flag.String("vsc", "my-volumesnapshotclass", "VolumeSnapshotClass name to use")
	awsID := flag.String("awsid", "", "AWS_ACCESS_KEY_ID for restic")
	awsSecret := flag.String("awssecret", "", "AWS_SECRET_ACCESS_KEY for restic")
	repository := flag.String("repository", "", "RESTIC_REPOSITORY value")
	password := flag.String("password", "", "RESTIC_PASSWORD value")
	// For restore mode, sourcePV must be provided.
	sourcePV := flag.String("sourcepv", "", "Original source PV name used for the backup (required in restore mode)")

	flag.Parse()

	if *mode != "backup" && *mode != "restore" {
		log.Fatal("❌ Please specify -mode=backup or -mode=restore")
	}
	if *pvcName == "" {
		log.Fatal("❌ Please provide the PVC name using -pvc flag")
	}
	if *mode == "restore" && *sourcePV == "" {
		log.Fatal("❌ Please provide the source PV name using -sourcepv flag in restore mode")
	}
	if *awsID == "" || *awsSecret == "" || *repository == "" || *password == "" {
		log.Fatal("❌ Please provide all secret parameters: -awsid, -awssecret, -repository, -password")
	}

	if err := k8s.InitK8sClients(*kubeconfig); err != nil {
		log.Fatalf("❌ Error initializing Kubernetes clients: %v", err)
	}

	// Run repository check job.
	log.Println("🔧 Applying repository check job manifest...")
	checkRepls := map[string]string{
		"AWS_ACCESS_KEY_ID":     *awsID,
		"AWS_SECRET_ACCESS_KEY": *awsSecret,
		"RESTIC_REPOSITORY":     *repository,
		"RESTIC_PASSWORD":       *password,
		"NAMESPACE":             *namespace,
	}
	checkManifest := k8s.ReplacePlaceholders(manifests.ResticCheckJob, checkRepls)
	if err := k8s.ApplyManifest(checkManifest, *namespace, "", true); err != nil {
		log.Fatalf("❌ Failed to apply repository check job manifest: %v", err)
	}
	log.Println("⌛ Waiting for repository check job to complete...")
	checkErr := k8s.WaitForJob("restic-check", *namespace, 10*time.Second)
	repoInitialized := (checkErr == nil)

	switch *mode {
	case "backup":
		backup.RunBackup(*namespace, *pvcName, *vsc, *awsID, *awsSecret, *repository, *password, repoInitialized)
	case "restore":
		restore.RunRestore(*namespace, *pvcName, *sourcePV, *awsID, *awsSecret, *repository, *password, repoInitialized)
	default:
		log.Fatal("❌ Invalid mode specified. Use 'backup' or 'restore'.")
	}
}
