#!/bin/bash

while [ ! -f /data/capture_finished ]; do
  sleep 2
done

aws s3 cp /data/dump.pcap s3://${BUCKET_NAME}/${FILE_NAME} \
    --endpoint-url https://storage.yandexcloud.net \
    --region ru-central1

echo "Starting upload to Yandex Cloud..."

if [ $? -eq 0 ]; then
  echo "Upload successful"
  exit 0
else
  echo "Upload failed"
  exit 1
fi