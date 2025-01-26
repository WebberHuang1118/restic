FROM ubuntu:22.04

ENV RESTIC_VERSION=0.15.0
ENV RESTIC_DOWNLOAD_URL=https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2

# Install required tools
RUN apt-get update \
    && apt-get install -y curl bzip2 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Download and uncompress restic
RUN curl -L ${RESTIC_DOWNLOAD_URL} -o /tmp/restic.bz2 \
    && bzip2 -d /tmp/restic.bz2 \
    && chmod +x /tmp/restic \
    && mv /tmp/restic /usr/local/bin/restic

CMD ["restic", "version"]
