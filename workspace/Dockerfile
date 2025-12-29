# ClaraTeach Learner Workspace Container
# Base image with Claude CLI, Python, Node.js, and dev tools

FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    # Build essentials
    build-essential \
    # Python
    python3.11 \
    python3.11-venv \
    python3-pip \
    # Core tools
    curl \
    wget \
    git \
    jq \
    vim \
    nano \
    # Terminal utilities
    tmux \
    htop \
    # Required for node-pty
    make \
    g++ \
    # Clean up
    && rm -rf /var/lib/apt/lists/*

# Install Node.js 20 LTS
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -m -s /bin/bash -u 1000 learner \
    && mkdir -p /workspace \
    && chown -R learner:learner /workspace

# Set Python 3.11 as default
RUN update-alternatives --install /usr/bin/python python /usr/bin/python3.11 1 \
    && update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1

# Install Claude CLI globally (as root, before switching user)
RUN npm install -g @anthropic-ai/claude-code

# Install pino-pretty for log formatting
RUN npm install -g pino-pretty

# Copy server code and install dependencies (as root for node-pty build)
COPY --chown=learner:learner server /home/learner/server
WORKDIR /home/learner/server
RUN npm install && npm run build

# Switch to non-root user
USER learner
WORKDIR /home/learner

# Copy configuration files
COPY --chown=learner:learner config/tmux.conf /home/learner/.tmux.conf
COPY --chown=learner:learner scripts/entrypoint.sh /home/learner/entrypoint.sh

# Make entrypoint executable
RUN chmod +x /home/learner/entrypoint.sh

# Set working directory to workspace
WORKDIR /workspace

# Environment variables
ENV HOME=/home/learner
ENV PATH="/home/learner/.local/bin:/home/learner/.npm-global/bin:${PATH}"
ENV WORKSPACE_DIR=/workspace
ENV TERM=xterm-256color

# Expose workspace server port
EXPOSE 3000

# Default command - start tmux session
ENTRYPOINT ["/home/learner/entrypoint.sh"]
CMD ["bash"]
