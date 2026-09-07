#!/bin/sh
systemctl daemon-reload || true
systemctl enable nrlnanny.service || true
systemctl restart nrlnanny.service || true
