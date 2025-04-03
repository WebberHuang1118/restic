package restore

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yourusername/restic-backup/pkg/k8s"
	"github.com/yourusername/restic-backup/pkg/manifests"
)

// ReplacePlaceholders is a local helper.
func ReplacePlaceholders(manifest string, replacements map[string]string) string {
	for key, value := range replacements {
		placeholder := fmt.Sprintf("{{%s}}", key)
		manifest = strings.ReplaceAll(manifest, placeholder, value)
	}
	return manifest
}

// RunRestore executes the restore workflow.
func RunRestore(namespace, destPVC, sourcePV, awsID, awsSecret, repository, password string, repoInitialized bool) {
	if !repoInitialized {
		log.Fatal("❌ Restic repository is not initialized. Cannot perform restore.")
	}
	log.Println("🔧 Applying restore job manifest...")
	restoreRepls := map[string]string{
		"PVC_NAME":              destPVC,
		"NAMESPACE":             namespace,
		"PV_NAME":               sourcePV,
		"AWS_ACCESS_KEY_ID":     awsID,
		"AWS_SECRET_ACCESS_KEY": awsSecret,
		"RESTIC_REPOSITORY":     repository,
		"RESTIC_PASSWORD":       password,
	}
	restoreManifest := k8s.ReplacePlaceholders(manifests.RestoreJob, restoreRepls)
	if err := k8s.ApplyManifest(restoreManifest, namespace, destPVC, true); err != nil {
		log.Fatalf("❌ Failed to apply restore job manifest: %v", err)
	}

	// Launch log streaming to capture restore progress from the "restore" container.
	go func() {
		if err := k8s.StreamJobProgressPercentage("block-restore-job", namespace, "restore", "WRITE progress:"); err != nil {
			log.Printf("❌ Error streaming restore progress logs: %v", err)
		}
	}()

	log.Println("⌛ Waiting for restore job to complete...")
	if err := k8s.WaitForJob("block-restore-job", namespace, 3600*time.Second); err != nil {
		log.Fatalf("❌ Restore job did not complete: %v", err)
	}
	log.Println("✅ Restore completed successfully.")
}
