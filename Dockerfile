FROM alpine:latest

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create app user
RUN adduser -D -u 1000 release-engine

# Copy the release-engine binary
COPY release-engine /usr/local/bin/release-engine

# Copy module/config files required for config-managed bootstrap
COPY config /etc/release-engine/config

# Set permissions
RUN chmod +x /usr/local/bin/release-engine

# Switch to non-root user
USER release-engine

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD [ "release-engine", "db", "ping" ] || exit 1

# Default command
CMD ["release-engine", "serve"]