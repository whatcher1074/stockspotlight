#!/bin/bash
# File: scripts/alert.sh
# Basic alert stub — expand later with email or webhook support

echo "[ALERT] Rate limit hit or API failure at $(date)" >> logs/app.log
