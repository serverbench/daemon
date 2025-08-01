FROM alpine

# Accept build architecture from buildx
ARG TARGETARCH

# Install dependencies
RUN apk update && apk add --no-cache tini openssh go shadow iproute2 iptables iptables-legacy ip6tables git rsync

# Configure sshd_config to use keys from /keys
RUN addgroup -S serverbench && \
    mkdir -p /users /var/run/sshd && \
    chown root:serverbench /users && \
    chmod 755 /users && \
    echo 'Port 23' >> /etc/ssh/sshd_config && \
    echo 'Subsystem sftp internal-sftp' >> /etc/ssh/sshd_config && \
    echo 'HostKey /keys/ssh_host_ed25519_key' >> /etc/ssh/sshd_config && \
    echo 'HostKey /keys/ssh_host_rsa_key' >> /etc/ssh/sshd_config && \
    echo 'Match Group serverbench' >> /etc/ssh/sshd_config && \
    echo '  ForceCommand internal-sftp -d /data' >> /etc/ssh/sshd_config && \
    echo '  ChrootDirectory /users/%u' >> /etc/ssh/sshd_config && \
    echo "root:root" | chpasswd && \
    sed -i 's/#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config

# Copy and build app
COPY . /build
WORKDIR /build

# Use architecture-specific GOARCH
RUN GOOS=linux GOARCH=$TARGETARCH go build -o /app/serverbench

WORKDIR /app

# Entrypoint script to init keys if needed
COPY ./wrapper /wrapper
RUN chmod +x /wrapper/iptables
RUN chmod +x /wrapper/ip6tables
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22

ENTRYPOINT ["/entrypoint.sh"]
