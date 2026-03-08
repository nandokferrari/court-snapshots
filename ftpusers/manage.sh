#!/bin/bash
# Usage: ./manage.sh add <court-uuid> <password>
#        ./manage.sh remove <court-uuid>

ACTION=$1
COURT_ID=$2
PASSWORD=$3
BASE_DIR="/snapshots"
USERNAME="cam_${COURT_ID:0:8}"

case $ACTION in
  add)
    mkdir -p "$BASE_DIR/court-$COURT_ID"
    useradd -d "$BASE_DIR/court-$COURT_ID" -s /usr/sbin/nologin "$USERNAME"
    echo "$USERNAME:$PASSWORD" | chpasswd
    chown "$USERNAME:$USERNAME" "$BASE_DIR/court-$COURT_ID"
    echo "$USERNAME" >> /etc/vsftpd.userlist
    echo "User $USERNAME created for court $COURT_ID"
    ;;
  remove)
    userdel "$USERNAME"
    sed -i "/$USERNAME/d" /etc/vsftpd.userlist
    echo "User $USERNAME removed"
    ;;
  *)
    echo "Usage: $0 {add|remove} <court-uuid> [password]"
    exit 1
    ;;
esac
