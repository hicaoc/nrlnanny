#!/bin/sh
rc-update add nrlnanny default || true
rc-service nrlnanny restart || true
