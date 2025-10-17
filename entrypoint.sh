#!/bin/sh

# Generate SSH host keys in /keys if missing
if [ ! -f /keys/ssh_host_ed25519_key ]; then
    echo "Generating ED25519 host key..."
    ssh-keygen -t ed25519 -f /keys/ssh_host_ed25519_key -N ""
fi

if [ ! -f /keys/ssh_host_rsa_key ]; then
    echo "Generating RSA host key..."
    ssh-keygen -t rsa -b 4096 -f /keys/ssh_host_rsa_key -N ""
fi

# Start sshd in background and run app
exec /app/serverbench