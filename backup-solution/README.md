# Backup Solution for Block-Mode PVCs with Restic

This repository provides a complete backup solution for Kubernetes block-mode PVCs using Restic. The solution consists of four jobs:

1. **init-restic-repo.yaml**  
   Initializes the Restic repository if it is not already initialized.

2. **check-backup-mode.yaml**  
   Checks the block-mode PVC (exposed as `/dev/block-device`) and determines whether a raw backup or a file-level backup is appropriate.  
   - It prints "raw" if a partition table exists or if the filesystem is not a recognized Linux type.
   - It prints "file" if a single Linux filesystem is detected.

3. **raw-backup-job.yaml**  
   Performs a raw, bit‑for‑bit backup using `dd` piped into Restic.

4. **file-backup-job.yaml**  
   Mounts the block device (if not already mounted) to back up the filesystem using Restic in file-level mode.

All jobs use the secret **restic-env** (defined in `secret-restic-env.yaml`) to load required environment variables.

## Directory Structure

