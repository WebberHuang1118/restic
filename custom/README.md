1. Make sure s3:http://192.188.0.50:9000/restic-testing is empty
2. $ kubectl apply -f restic-init.yaml
3. $ kubectl apply -f restic-backup.yaml
4. $ kubectl apply -f restic-restore.yaml

Testing restic
$ kubectl apply -f restic-testing.yaml
and entor the job's pod to check restic command