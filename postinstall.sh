#/bin/bash
mkdir -p /opt/assimilator/bin
mv /usr/bin/assimilator-launcher /opt/assimilator/bin/assimilator-launcher
systemctl enable assimilator
systemctl start assimilator