Backup mode:
    $ ./bin/restic-backup -awsid minioadmin -awssecret minioadmin -password abc -repository s3:http://192.188.0.56:9000/restic-testing -mode backup -pvc vol1 -vsc lvm-snapshotd

Restore mode:
    $ ./bin/restic-backup -awsid minioadmin -awssecret minioadmin -password abc -repository s3:http://192.188.0.56:9000/restic-testing -mode restore -pvc vol2 -sourcepv pvc-c42288e1-aa3e-4022-8e2c-f426b14099dd