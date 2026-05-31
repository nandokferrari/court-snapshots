#!/bin/bash
# Inject user_config_dir into the generated config before vsftpd starts
sed -i '/\/usr\/sbin\/vsftpd/i echo "user_config_dir=/etc/vsftpd/user_conf" >> /etc/vsftpd/vsftpd.conf' /usr/sbin/run-vsftpd.sh
exec /usr/sbin/run-vsftpd.sh
