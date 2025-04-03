package manifests

// MinioCredentials (optional). Secrets are passed via job commands.
const MinioCredentials = `
apiVersion: v1
kind: Secret
metadata:
  name: minio-credentials
  namespace: {{NAMESPACE}}
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: "minioadmin"
  AWS_SECRET_ACCESS_KEY: "minioadmin"
  RESTIC_REPOSITORY: "s3:http://192.188.0.56:9000/restic-testing"
  RESTIC_PASSWORD: "abc"
`

// ResticCheckJob checks if the repository is initialized.
const ResticCheckJob = `
apiVersion: batch/v1
kind: Job
metadata:
  name: restic-check
  namespace: {{NAMESPACE}}
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 30
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: restic-check
        image: webberhuang/restic-accelerated:latest
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c"]
        args:
          - export AWS_ACCESS_KEY_ID={{AWS_ACCESS_KEY_ID}} && export AWS_SECRET_ACCESS_KEY={{AWS_SECRET_ACCESS_KEY}} && export RESTIC_REPOSITORY={{RESTIC_REPOSITORY}} && export RESTIC_PASSWORD={{RESTIC_PASSWORD}} && restic snapshots > /dev/null 2>&1
`

// ResticInitJob initializes the repository.
const ResticInitJob = `
apiVersion: batch/v1
kind: Job
metadata:
  name: restic-init
  namespace: {{NAMESPACE}}
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 30
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: restic-init
        image: webberhuang/restic-accelerated:latest
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c"]
        args:
          - export AWS_ACCESS_KEY_ID={{AWS_ACCESS_KEY_ID}} && export AWS_SECRET_ACCESS_KEY={{AWS_SECRET_ACCESS_KEY}} && export RESTIC_REPOSITORY={{RESTIC_REPOSITORY}} && export RESTIC_PASSWORD={{RESTIC_PASSWORD}} && restic init
`

// VolumeSnapshot creates a snapshot from a given PVC.
const VolumeSnapshot = `
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: {{VOLUME_SNAPSHOT_NAME}}
  namespace: {{NAMESPACE}}
spec:
  volumeSnapshotClassName: {{VOLUME_SNAPSHOT_CLASSNAME}}
  source:
    persistentVolumeClaimName: {{PVC_NAME}}
`

// PVCClone creates a new PVC from a VolumeSnapshot.
const PVCClone = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{NEW_PVC_NAME}}
  namespace: {{NAMESPACE}}
spec:
  accessModes:
  - ReadWriteOnce
  volumeMode: {{VOLUME_MODE}}
  storageClassName: {{STORAGE_CLASS}}
  resources:
    requests:
      storage: {{STORAGE_SIZE}}
  dataSource:
    name: {{VOLUME_SNAPSHOT_NAME}}
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
`

// BackupJob defines the backup job.
const BackupJob = `
apiVersion: batch/v1
kind: Job
metadata:
  name: block-backup-job
  namespace: {{NAMESPACE}}
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 30
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: backup
        image: webberhuang/restic-accelerated:latest
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c"]
        args:
          - export AWS_ACCESS_KEY_ID={{AWS_ACCESS_KEY_ID}} && export AWS_SECRET_ACCESS_KEY={{AWS_SECRET_ACCESS_KEY}} && export RESTIC_REPOSITORY={{RESTIC_REPOSITORY}} && export RESTIC_PASSWORD={{RESTIC_PASSWORD}} && /usr/local/bin/accelerated_io -device /dev/{{PVC_NAME}} -mode=read | restic -q backup --stdin --stdin-filename {{PV_NAME}}.img
        volumeDevices:
        - name: vol1
          devicePath: /dev/{{PVC_NAME}}
      volumes:
      - name: vol1
        persistentVolumeClaim:
          claimName: "{{PVC_NAME}}"
`

// RestoreJob defines the restore job.
const RestoreJob = `
apiVersion: batch/v1
kind: Job
metadata:
  name: block-restore-job
  namespace: {{NAMESPACE}}
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 30
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: restore
        image: webberhuang/restic-accelerated:latest
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c"]
        args:
          - export AWS_ACCESS_KEY_ID={{AWS_ACCESS_KEY_ID}} && export AWS_SECRET_ACCESS_KEY={{AWS_SECRET_ACCESS_KEY}} && export RESTIC_REPOSITORY={{RESTIC_REPOSITORY}} && export RESTIC_PASSWORD={{RESTIC_PASSWORD}} && restic --verbose=2 dump latest {{PV_NAME}}.img | /usr/local/bin/accelerated_io -device /dev/{{PVC_NAME}} -mode=write
        volumeDevices:
        - name: vol2
          devicePath: /dev/{{PVC_NAME}}
      volumes:
      - name: vol2
        persistentVolumeClaim:
          claimName: "{{PVC_NAME}}"
`
