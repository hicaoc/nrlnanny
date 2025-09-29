#!/bin/bash

cp nrlnanny.service /lib/systemd/system/

systemctl daemon-reload
systemctl enable nrlnanny
systemctl start nrlnanny
