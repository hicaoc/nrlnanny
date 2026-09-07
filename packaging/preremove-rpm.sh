#!/bin/sh
systemctl stop nrlnanny.service || true
systemctl disable nrlnanny.service || true
