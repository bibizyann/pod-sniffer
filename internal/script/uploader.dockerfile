FROM amazon/aws-cli:latest

COPY uploader.sh /uploader.sh
RUN chmod +x /uploader.sh

ENTRYPOINT [ "/uploader.sh" ]