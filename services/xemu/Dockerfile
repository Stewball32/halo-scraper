# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1: Build halo-scraper (Halo CE per-tick memory scraper)
# =============================================================================
FROM golang:1.22-bookworm AS scraper-build
WORKDIR /src
COPY halo-scraper/go.mod halo-scraper/go.sum ./
RUN go mod download
COPY halo-scraper/ ./
RUN CGO_ENABLED=0 go build -o /scraper ./cmd/scraper

# =============================================================================
# Stage 2: xemu overlay with halo-scraper embedded
# =============================================================================
FROM lscr.io/linuxserver/xemu:latest

# set version label
LABEL maintainer="Roasted-Codes"

# Install additional packages for window automation, networking, and pcap shim compilation
RUN \
  apt-get update && \
  apt-get install -y --no-install-recommends \
    wmctrl \
    ethtool \
    gcc \
    libpcap-dev && \
  apt-get autoclean && \
  rm -rf \
    /var/lib/apt/lists/* \
    /var/tmp/* \
    /tmp/*

# Compile pcap immediate mode shim into the image.
# Without this, xemu's pcap backend can send packets but never receives them
# (libpcap >= 1.9 with TPACKET_V3: the selectable fd never becomes readable
# without immediate mode). The shim intercepts pcap_open_live() and injects
# pcap_set_immediate_mode(1). Baked into the image so it survives volume wipes.
COPY cairo-station-LAN/services/xemu/data/emulator/pcap_immediate.c /tmp/pcap_immediate.c
RUN gcc -shared -fPIC -o /opt/pcap_immediate.so /tmp/pcap_immediate.c -lpcap && \
    rm /tmp/pcap_immediate.c

# Replace upstream autostart with custom version (dual QMP: unix + tcp)
COPY cairo-station-LAN/services/xemu/root/defaults/autostart /defaults/autostart

# halo-scraper binary + web assets
COPY --from=scraper-build /scraper /opt/halo-scraper/scraper
COPY halo-scraper/web /opt/halo-scraper/web
