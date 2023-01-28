## Pseudo Code
- get source folder and the destination folder from settings
- recursively loop over only directories within the source folder and add them to a list
- do the following for each subdirectory
  - create a destination folder name based on source directory
  - get all keys for files in s3 for the destination folder (this can be done in parallel to speed up the listobjects call since it limits to 1000keys)
  - if keys don't exist no comparison needed
    - loop over all files
    - compress (optionally encrypt) and upload file
  - if keys exist we need to compare files
    - loop over all files
    - compare file last modified timestamp with object's last modified time
    - possibly add size check as well using gzip header size
    - compress (optionally encrypt) and upload file

***Make sure to add content-type to allow direct visualization from S3*