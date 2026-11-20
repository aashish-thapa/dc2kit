#!/bin/sh
# Brings a devcontainer base image up to the sandbox platform floor: bash,
# git, a CA store, sudo, and the `agent` account at uid 1000 with its home at
# /home/agent, which is where published agent mixins write. Runs once, as
# root, at image build.
set -eu

missing=""
need_command() { command -v "$1" >/dev/null 2>&1 || missing="$missing $1"; }
need_command bash
need_command git
need_command sudo
need_command usermod
if [ ! -s /etc/ssl/certs/ca-certificates.crt ] && [ ! -s /etc/pki/tls/certs/ca-bundle.crt ]; then
  missing="$missing ca-certificates"
fi

package_manager() {
  for candidate in apt-get apk dnf microdnf yum; do
    if command -v "$candidate" >/dev/null 2>&1; then
      echo "$candidate"
      return
    fi
  done
}

# Maps a missing command onto the package that provides it.
package_for() {
  case "$1:$2" in
    apt-get:usermod) echo passwd ;;
    apk:usermod) echo shadow ;;
    dnf:usermod | microdnf:usermod | yum:usermod) echo shadow-utils ;;
    *) echo "$2" ;;
  esac
}

if [ -n "$missing" ]; then
  manager=$(package_manager)
  if [ -z "$manager" ]; then
    echo "dc2kit: the base image lacks:$missing, and has no apt-get, apk, dnf, microdnf or yum to install them" >&2
    exit 1
  fi
  packages=""
  for item in $missing; do
    packages="$packages $(package_for "$manager" "$item")"
  done
  case "$manager" in
    apt-get)
      apt-get update
      # shellcheck disable=SC2086
      DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends $packages
      rm -rf /var/lib/apt/lists/*
      ;;
    apk)
      # shellcheck disable=SC2086
      apk add --no-cache $packages
      ;;
    *)
      # shellcheck disable=SC2086
      "$manager" install -y $packages
      "$manager" clean all
      ;;
  esac
fi

group_1000=$(awk -F: '$3 == 1000 { print $1; exit }' /etc/group)
if [ -z "$group_1000" ]; then
  groupadd --gid 1000 agent
elif [ "$group_1000" != agent ]; then
  groupmod --new-name agent "$group_1000"
fi

user_1000=$(awk -F: '$3 == 1000 { print $1; exit }' /etc/passwd)
if [ -z "$user_1000" ]; then
  useradd --uid 1000 --gid 1000 --create-home --home-dir /home/agent --shell /bin/bash agent
elif [ "$user_1000" != agent ]; then
  old_home=$(awk -F: -v user="$user_1000" '$1 == user { print $6; exit }' /etc/passwd)
  usermod --login agent --home /home/agent --move-home --shell /bin/bash "$user_1000"
  # Images and features often hardcode the old home; keep it resolving.
  if [ -n "$old_home" ] && [ "$old_home" != /home/agent ] && [ ! -e "$old_home" ]; then
    ln -s /home/agent "$old_home"
  fi
else
  usermod --shell /bin/bash agent
fi

mkdir -p /etc/sudoers.d
printf 'agent ALL=(ALL) NOPASSWD:ALL\n' > /etc/sudoers.d/agent
chmod 0440 /etc/sudoers.d/agent

mkdir -p /home/agent/workspace /usr/local/share/ca-certificates /etc/profile.d
chown 1000:1000 /home/agent /home/agent/workspace
touch /etc/sandbox-persistent.sh
printf '. /etc/sandbox-persistent.sh\n' > /etc/profile.d/sandbox-persistent.sh
