#!/bin/sh
rc-service nrlnanny stop || true
rc-update del nrlnanny default || true
