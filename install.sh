#!/usr/bin/env bash
set -e

# Download and install the latest release binary.
#
# To install:
#
# ```
# ./install.sh path/to/bin/directory
# ```
#
# or to install at `/usr/local/bin`
#
# ```
# sudo ./install.sh
# ```

ARCH=$(uname -sm)
BIN_DIR=${1:-/usr/local/bin}

case $ARCH in
    "Linux x86_64")
        TARGET="linux-amd64"
        ;;
    "Linux aarch64")
        TARGET="linux-arm64"
        ;;
    "Linux armv7l")
        TARGET="linux-arm"
        ;;
esac

if [ -z "$TARGET" ]; then
    echo "Unknown target architecture: $ARCH" >&2
    exit 1
fi

if [ ! -d "$BIN_DIR" ]; then
    echo "Can't find the target install directory: $BIN_DIR" >&2
    exit 1
fi

GOAT_COUNTER_VERSION=$(curl -s https://api.github.com/repos/arp242/goatcounter/releases/latest | grep tag_name  | grep -Eo "v[0-9]+\.[0-9]+\.[0-9]+")
DOWNLOAD_PATH="https://github.com/arp242/goatcounter/releases/download/$GOAT_COUNTER_VERSION/goatcounter-$GOAT_COUNTER_VERSION-$TARGET.gz"

curl -s -L $DOWNLOAD_PATH  | gunzip > $BIN_DIR/goatcounter
chmod +x $BIN_DIR/goatcounter
