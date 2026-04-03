#!/bin/bash
#
# bootstrap-proot.sh
# 
# Downloads Alpine Linux 3.20 rootfs for ARM64, verifies SHA256 checksum,
# extracts to the rootfs directory, and sets up the initial PocketServer
# directory structure inside the proot environment.
#
# This script runs on Android via Termux or within the Flutter app's native context.
# It must handle network interruptions gracefully and support resume downloads.
#
# Usage: ./bootstrap-proot.sh [target_directory]
#   target_directory: Optional. Defaults to current directory.
#                     Should be /data/data/com.pocketserver.app/
#
# Exit codes:
#   0 - Success
#   1 - Invalid arguments or missing dependencies
#   2 - Download failed
#   3 - Checksum verification failed
#   4 - Extraction failed
#   5 - Directory structure creation failed

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

readonly ALPINE_VERSION="3.20"
readonly ALPINE_ARCH="aarch64"
readonly ALPINE_MIRROR="https://dl-cdn.alpinelinux.org/alpine"
readonly ALPINE_ROOTFS_URL="${ALPINE_MIRROR}/v${ALPINE_VERSION}/releases/${ALPINE_ARCH}/alpine-minirootfs-${ALPINE_VERSION}.0-${ALPINE_ARCH}.tar.gz"
readonly ALPINE_CHECKSUM_URL="${ALPINE_MIRROR}/v${ALPINE_VERSION}/releases/${ALPINE_ARCH}/alpine-minirootfs-${ALPINE_VERSION}.0-${ALPINE_ARCH}.tar.gz.sha256"

# PocketServer directory structure (relative to target directory)
readonly ROOTFS_DIR="rootfs"
readonly PROJECTS_DIR="projects"
readonly BACKUPS_DIR="backups"
readonly CERTS_DIR="certs"
readonly CONFIG_FILE="config.json"

# Logging
readonly LOG_TAG="PocketServer.Bootstrap"

# =============================================================================
# Utility Functions
# =============================================================================

log_info() {
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] [INFO] [$LOG_TAG] $*"
}

log_error() {
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] [ERROR] [$LOG_TAG] $*" >&2
}

log_warn() {
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] [WARN] [$LOG_TAG] $*"
}

die() {
    log_error "$1"
    exit "${2:-1}"
}

# =============================================================================
# Dependency Checks
# =============================================================================

check_dependencies() {
    log_info "Checking dependencies..."
    
    local missing_deps=()
    
    # Check for required commands
    for cmd in curl sha256sum tar mkdir chmod; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_deps+=("$cmd")
        fi
    done
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        die "Missing required dependencies: ${missing_deps[*]}" 1
    fi
    
    log_info "All dependencies satisfied."
}

# =============================================================================
# Directory Setup
# =============================================================================

setup_base_directory() {
    local target_dir="$1"
    
    log_info "Setting up base directory: $target_dir"
    
    # Create target directory if it doesn't exist
    if [[ ! -d "$target_dir" ]]; then
        mkdir -p "$target_dir" || die "Failed to create target directory: $target_dir" 1
    fi
    
    # Change to target directory
    cd "$target_dir" || die "Failed to change to target directory: $target_dir" 1
    
    log_info "Working directory set to: $(pwd)"
}

create_pocketserver_structure() {
    local target_dir="$1"
    
    log_info "Creating PocketServer directory structure..."
    
    # Create main directories
    local dirs=(
        "$ROOTFS_DIR"
        "$PROJECTS_DIR"
        "$BACKUPS_DIR"
        "$CERTS_DIR"
        "$CERTS_DIR/trusted"
    )
    
    for dir in "${dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            mkdir -p "$dir" || die "Failed to create directory: $dir" 5
            log_info "Created directory: $dir"
        else
            log_warn "Directory already exists: $dir"
        fi
    done
    
    # Set appropriate permissions
    chmod 700 "$CERTS_DIR" || die "Failed to set permissions on certs directory" 5
    chmod 700 "$CERTS_DIR/trusted" || die "Failed to set permissions on trusted certs directory" 5
    
    log_info "Directory structure created successfully."
}

create_rootfs_substructure() {
    log_info "Creating rootfs subdirectory structure..."
    
    # Standard Linux filesystem hierarchy + PocketServer-specific paths
    local dirs=(
        "$ROOTFS_DIR/etc/nginx/sites-enabled"
        "$ROOTFS_DIR/etc/php8/php-fpm.d"
        "$ROOTFS_DIR/etc/haproxy"
        "$ROOTFS_DIR/var/lib/mysql"
        "$ROOTFS_DIR/var/log/nginx"
        "$ROOTFS_DIR/var/log/php"
        "$ROOTFS_DIR/var/log/mysql"
        "$ROOTFS_DIR/var/log/pocketd"
        "$ROOTFS_DIR/usr/local/pocketd/templates"
        "$ROOTFS_DIR/usr/local/pocketd/scripts"
        "$ROOTFS_DIR/tmp"
        "$ROOTFS_DIR/run"
    )
    
    for dir in "${dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            mkdir -p "$dir" || die "Failed to create rootfs directory: $dir" 5
        fi
    done
    
    # Set permissions for tmp and run
    chmod 1777 "$ROOTFS_DIR/tmp" || die "Failed to set permissions on tmp" 5
    chmod 755 "$ROOTFS_DIR/run" || die "Failed to set permissions on run" 5
    
    log_info "Rootfs subdirectory structure created successfully."
}

# =============================================================================
# Rootfs Download with Resume Support
# =============================================================================

download_rootfs() {
    log_info "Downloading Alpine ${ALPINE_VERSION} rootfs (${ALPINE_ARCH})..."
    
    local rootfs_tar="alpine-rootfs.tar.gz"
    local checksum_file="alpine-rootfs.tar.gz.sha256"
    
    # Check if partial download exists
    local download_args="--fail --silent --show-progress"
    if [[ -f "$rootfs_tar" ]]; then
        log_warn "Partial download found. Attempting to resume..."
        download_args="$download_args --continue"
    fi
    
    # Download the rootfs tarball
    if ! curl $download_args \
        -H "User-Agent: PocketServer-Bootstrap/1.0" \
        --connect-timeout 30 \
        --max-time 600 \
        -o "$rootfs_tar" \
        "$ALPINE_ROOTFS_URL"; then
        
        # Clean up partial download on failure
        rm -f "$rootfs_tar"
        die "Failed to download Alpine rootfs from $ALPINE_ROOTFS_URL" 2
    fi
    
    log_info "Rootfs download completed."
    
    # Download checksum file
    log_info "Downloading checksum file..."
    if ! curl --fail --silent --show-progress \
        --connect-timeout 30 \
        --max-time 60 \
        -o "$checksum_file" \
        "$ALPINE_CHECKSUM_URL"; then
        
        rm -f "$checksum_file"
        die "Failed to download checksum file from $ALPINE_CHECKSUM_URL" 2
    fi
    
    log_info "Checksum file downloaded."
}

# =============================================================================
# Checksum Verification
# =============================================================================

verify_checksum() {
    log_info "Verifying SHA256 checksum..."
    
    local rootfs_tar="alpine-rootfs.tar.gz"
    local checksum_file="alpine-rootfs.tar.gz.sha256"
    
    if [[ ! -f "$rootfs_tar" ]]; then
        die "Rootfs tarball not found. Download may have failed." 3
    fi
    
    if [[ ! -f "$checksum_file" ]]; then
        die "Checksum file not found. Download may have failed." 3
    fi
    
    # Extract expected checksum from file
    # Format: <checksum>  <filename>
    local expected_checksum
    expected_checksum=$(awk '{print $1}' "$checksum_file")
    
    if [[ -z "$expected_checksum" ]]; then
        die "Invalid checksum file format" 3
    fi
    
    log_info "Expected checksum: $expected_checksum"
    
    # Compute actual checksum
    local actual_checksum
    actual_checksum=$(sha256sum "$rootfs_tar" | awk '{print $1}')
    
    log_info "Actual checksum:   $actual_checksum"
    
    # Compare checksums
    if [[ "$expected_checksum" != "$actual_checksum" ]]; then
        log_error "Checksum mismatch!"
        log_error "This could indicate a corrupted download or tampered file."
        rm -f "$rootfs_tar" "$checksum_file"
        die "SHA256 checksum verification failed" 3
    fi
    
    log_info "Checksum verification successful."
}

# =============================================================================
# Rootfs Extraction
# =============================================================================

extract_rootfs() {
    log_info "Extracting rootfs tarball..."
    
    local rootfs_tar="alpine-rootfs.tar.gz"
    
    if [[ ! -f "$rootfs_tar" ]]; then
        die "Rootfs tarball not found" 4
    fi
    
    # Extract to rootfs directory
    if ! tar -xzf "$rootfs_tar" -C "$ROOTFS_DIR" 2>&1; then
        die "Failed to extract rootfs tarball" 4
    fi
    
    log_info "Rootfs extraction completed."
    
    # Clean up downloaded files
    log_info "Cleaning up temporary files..."
    rm -f "$rootfs_tar"
    rm -f "alpine-rootfs.tar.gz.sha256"
    
    log_info "Cleanup completed."
}

# =============================================================================
# Initial Configuration
# =============================================================================

create_initial_config() {
    log_info "Creating initial config.json..."
    
    # Generate a UUID v4 for device_id
    # Using /proc/sys/kernel/random/uuid if available, otherwise generate manually
    local device_id
    if [[ -f /proc/sys/kernel/random/uuid ]]; then
        device_id=$(cat /proc/sys/kernel/random/uuid)
    else
        # Fallback: generate a pseudo-UUID using available tools
        device_id=$(od -x /dev/urandom | head -1 | awk '{OFS="-"; print $2$3,$4,$5,$6,$7$8$9}')
    fi
    
    # Generate random hex strings for security fields
    local api_key_salt
    local jwt_secret
    api_key_salt=$(head -c 32 /dev/urandom | xxd -p | tr -d '\n')
    jwt_secret=$(head -c 32 /dev/urandom | xxd -p | tr -d '\n')
    
    cat > "$CONFIG_FILE" << EOF
{
  "version": "1.0",
  "device_id": "$device_id",
  "device_name": "PocketServer Device",
  "resources": {
    "ram_mb": 512,
    "storage_mb": 5120,
    "cpu_percent": 30,
    "ports": {
      "http": 8080,
      "https": 8443,
      "mysql": 3306,
      "redis": 6379,
      "haproxy_stats": 9000
    }
  },
  "stack": {
    "php": true,
    "nodejs": false,
    "redis": false,
    "python": false
  },
  "network": {
    "cloudflare_tunnel_token": null,
    "bind_localhost_only": true,
    "peer_relay_url": null
  },
  "replication": {
    "mode": "async",
    "sync_interval_ms": 500,
    "peers": []
  },
  "backup": {
    "schedule": "daily",
    "retention_days": 30,
    "destinations": []
  },
  "security": {
    "api_key_salt": "$api_key_salt",
    "jwt_secret": "$jwt_secret"
  }
}
EOF
    
    # Restrict permissions on config file (contains secrets)
    chmod 600 "$CONFIG_FILE" || die "Failed to set permissions on config.json" 5
    
    log_info "Initial config.json created with device_id: $device_id"
}

# =============================================================================
# Bootstrap Script for Inside proot
# =============================================================================

create_proot_bootstrap_script() {
    log_info "Creating proot entry point script..."
    
    cat > "$ROOTFS_DIR/usr/local/pocketd/scripts/entrypoint.sh" << 'ENTRYPOINT_EOF'
#!/bin/sh
#
# entrypoint.sh
#
# Entry point script for running inside the proot environment.
# This script is called when entering the proot environment and
# is responsible for starting pocketd and managing services.
#

set -e

POCKETD_BIN="/usr/local/pocketd/pocketd"
POCKETD_CONFIG="/data/data/com.pocketserver.app/config.json"
LOG_FILE="/var/log/pocketd/bootstrap.log"

log() {
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*" >> "$LOG_FILE"
    echo "$*"
}

log "Starting PocketServer proot environment..."

# Verify pocketd binary exists
if [ ! -x "$POCKETD_BIN" ]; then
    log "ERROR: pocketd binary not found at $POCKETD_BIN"
    exit 1
fi

# Verify config exists
if [ ! -f "$POCKETD_CONFIG" ]; then
    log "ERROR: config.json not found at $POCKETD_CONFIG"
    exit 1
fi

log "Configuration verified. Ready to start pocketd."

# Export environment variables
export POCKETSERVER_ROOT="/data/data/com.pocketserver.app"
export POCKETSERVER_CONFIG="$POCKETD_CONFIG"

# Start pocketd (will be managed by Android foreground service)
exec "$POCKETD_BIN"
ENTRYPOINT_EOF
    
    chmod +x "$ROOTFS_DIR/usr/local/pocketd/scripts/entrypoint.sh"
    
    log_info "Proot entry point script created."
}

# =============================================================================
# Verification
# =============================================================================

verify_installation() {
    log_info "Verifying installation..."
    
    local errors=0
    
    # Check rootfs directory structure
    local required_dirs=(
        "$ROOTFS_DIR/etc"
        "$ROOTFS_DIR/var"
        "$ROOTFS_DIR/usr"
        "$ROOTFS_DIR/tmp"
        "$ROOTFS_DIR/etc/nginx/sites-enabled"
        "$ROOTFS_DIR/etc/php8/php-fpm.d"
        "$ROOTFS_DIR/etc/haproxy"
        "$ROOTFS_DIR/var/lib/mysql"
        "$ROOTFS_DIR/usr/local/pocketd/templates"
        "$ROOTFS_DIR/usr/local/pocketd/scripts"
    )
    
    for dir in "${required_dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            log_error "Missing directory: $dir"
            ((errors++))
        fi
    done
    
    # Check config file
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_error "Missing config.json"
        ((errors++))
    fi
    
    # Check entrypoint script
    if [[ ! -x "$ROOTFS_DIR/usr/local/pocketd/scripts/entrypoint.sh" ]]; then
        log_error "Entrypoint script not executable"
        ((errors++))
    fi
    
    # Check directory permissions
    if [[ ! -r "$CERTS_DIR" ]] || [[ "$(stat -c %a "$CERTS_DIR" 2>/dev/null)" != "700" ]]; then
        log_error "Certs directory has incorrect permissions"
        ((errors++))
    fi
    
    if [[ $errors -gt 0 ]]; then
        die "Verification failed with $errors error(s)" 5
    fi
    
    log_info "Installation verified successfully."
}

# =============================================================================
# Main Execution
# =============================================================================

main() {
    local target_dir="${1:-.}"
    
    echo "========================================"
    echo "  PocketServer proot Bootstrap"
    echo "  Alpine ${ALPINE_VERSION} (${ALPINE_ARCH})"
    echo "========================================"
    echo ""
    
    log_info "Starting bootstrap process..."
    log_info "Target directory: $(realpath "$target_dir")"
    
    # Step 1: Check dependencies
    check_dependencies
    
    # Step 2: Setup base directory
    setup_base_directory "$target_dir"
    
    # Step 3: Create PocketServer directory structure
    create_pocketserver_structure "$target_dir"
    
    # Step 4: Download rootfs
    download_rootfs
    
    # Step 5: Verify checksum
    verify_checksum
    
    # Step 6: Extract rootfs
    extract_rootfs
    
    # Step 7: Create rootfs substructure
    create_rootfs_substructure
    
    # Step 8: Create initial config
    create_initial_config
    
    # Step 9: Create proot bootstrap script
    create_proot_bootstrap_script
    
    # Step 10: Verify installation
    verify_installation
    
    echo ""
    echo "========================================"
    echo "  Bootstrap Complete!"
    echo "========================================"
    echo ""
    log_info "PocketServer proot environment is ready."
    log_info "Next steps:"
    log_info "  1. Install pocketd binary to: $ROOTFS_DIR/usr/local/pocketd/pocketd"
    log_info "  2. Configure Nginx, PHP-FPM, and MariaDB templates"
    log_info "  3. Start the Flutter app to complete setup"
    echo ""
    
    return 0
}

# Run main function with all arguments
main "$@"
