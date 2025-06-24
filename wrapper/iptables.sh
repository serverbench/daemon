#!/bin/sh
exec nsenter --net=/mnt/host_netns /sbin/iptables "$@"