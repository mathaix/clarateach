// Package rootfs provides utilities for building Firecracker rootfs images.
// It creates ext4 filesystem images from Docker containers with an init script
// for Firecracker MicroVMs.
package rootfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// Config holds the configuration for building a rootfs image.
type Config struct {
	// DockerImage is the name of the Docker image to export (e.g., "clarateach-workspace")
	DockerImage string

	// DockerfilePath is the path to a Dockerfile to build (optional)
	// If set and the image doesn't exist, it will be built
	DockerfilePath string

	// DockerContext is the build context directory (defaults to Dockerfile's directory)
	DockerContext string

	// OutputPath is the destination path for the rootfs.ext4 file
	OutputPath string

	// Size is the size of the rootfs image (e.g., "2G", "4G")
	// Defaults to "2G"
	Size string

	// InitScript is a custom init script content (optional)
	// If empty, the default init script will be used
	InitScript string
}

// Builder creates rootfs images for Firecracker MicroVMs.
type Builder struct {
	logger *logrus.Logger
}

// NewBuilder creates a new rootfs Builder.
func NewBuilder() *Builder {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	return &Builder{logger: logger}
}

// SetLogLevel sets the logging level.
func (b *Builder) SetLogLevel(level logrus.Level) {
	b.logger.SetLevel(level)
}

// Build creates a rootfs image from a Docker container.
func (b *Builder) Build(ctx context.Context, cfg Config) error {
	// Validate and set defaults
	if err := b.validateConfig(&cfg); err != nil {
		return err
	}

	// Check dependencies
	if err := b.checkDependencies(); err != nil {
		return err
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rootfs-build-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer b.cleanup(tmpDir)

	b.logger.Infof("Using temp directory: %s", tmpDir)

	// Step 1: Ensure Docker image exists
	if err := b.ensureDockerImage(ctx, cfg); err != nil {
		return fmt.Errorf("failed to ensure Docker image: %w", err)
	}

	// Step 2: Create and format ext4 file
	rootfsPath := filepath.Join(tmpDir, "rootfs.ext4")
	if err := b.createExt4(ctx, rootfsPath, cfg.Size); err != nil {
		return fmt.Errorf("failed to create ext4 image: %w", err)
	}

	// Step 3: Mount ext4 file
	mountPoint := filepath.Join(tmpDir, "mnt")
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}
	if err := b.mount(ctx, rootfsPath, mountPoint); err != nil {
		return fmt.Errorf("failed to mount rootfs: %w", err)
	}
	defer b.unmount(mountPoint)

	// Step 4: Export Docker container to mounted filesystem
	if err := b.exportDocker(ctx, cfg.DockerImage, mountPoint); err != nil {
		return fmt.Errorf("failed to export Docker image: %w", err)
	}

	// Step 5: Inject init script
	initScript := cfg.InitScript
	if initScript == "" {
		initScript = DefaultInitScript
	}
	if err := b.injectInitScript(mountPoint, initScript); err != nil {
		return fmt.Errorf("failed to inject init script: %w", err)
	}

	// Step 6: Ensure required utilities exist
	if err := b.ensureUtilities(ctx, cfg.DockerImage, mountPoint); err != nil {
		b.logger.Warnf("Failed to ensure utilities: %v", err)
	}

	// Step 7: Unmount
	if err := b.unmount(mountPoint); err != nil {
		return fmt.Errorf("failed to unmount rootfs: %w", err)
	}

	// Step 8: Move to output path
	if err := b.moveOutput(rootfsPath, cfg.OutputPath); err != nil {
		return fmt.Errorf("failed to move output: %w", err)
	}

	b.logger.Infof("Rootfs build complete: %s", cfg.OutputPath)
	return nil
}

func (b *Builder) validateConfig(cfg *Config) error {
	if cfg.DockerImage == "" {
		return fmt.Errorf("DockerImage is required")
	}
	if cfg.OutputPath == "" {
		return fmt.Errorf("OutputPath is required")
	}
	if cfg.Size == "" {
		cfg.Size = "2G"
	}
	return nil
}

func (b *Builder) checkDependencies() error {
	deps := []string{"docker"}
	for _, dep := range deps {
		if _, err := exec.LookPath(dep); err != nil {
			return fmt.Errorf("%s is not installed or not in PATH", dep)
		}
	}

	// Check for mkfs.ext4 (may be in /usr/sbin)
	mkfsPath := b.findMkfsExt4()
	if mkfsPath == "" {
		return fmt.Errorf("mkfs.ext4 is not installed (install e2fsprogs)")
	}

	return nil
}

func (b *Builder) findMkfsExt4() string {
	paths := []string{
		"mkfs.ext4",
		"/usr/sbin/mkfs.ext4",
		"/sbin/mkfs.ext4",
	}
	for _, p := range paths {
		if path, err := exec.LookPath(p); err == nil {
			return path
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (b *Builder) cleanup(tmpDir string) {
	b.logger.Info("Cleaning up...")
	// Try to unmount any mounted filesystems
	mntPath := filepath.Join(tmpDir, "mnt")
	b.unmount(mntPath)
	os.RemoveAll(tmpDir)
}

func (b *Builder) ensureDockerImage(ctx context.Context, cfg Config) error {
	// Check if image exists
	cmd := exec.CommandContext(ctx, "docker", "images", "-q", cfg.DockerImage)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Docker image: %w", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		b.logger.Infof("Docker image %s found", cfg.DockerImage)
		return nil
	}

	// Image doesn't exist, try to build it
	if cfg.DockerfilePath == "" {
		return fmt.Errorf("Docker image %s not found and no Dockerfile specified", cfg.DockerImage)
	}

	b.logger.Infof("Building Docker image %s from %s", cfg.DockerImage, cfg.DockerfilePath)

	dockerContext := cfg.DockerContext
	if dockerContext == "" {
		dockerContext = filepath.Dir(cfg.DockerfilePath)
	}

	cmd = exec.CommandContext(ctx, "docker", "build",
		"-t", cfg.DockerImage,
		"-f", cfg.DockerfilePath,
		dockerContext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	return nil
}

func (b *Builder) createExt4(ctx context.Context, path, size string) error {
	b.logger.Infof("Creating ext4 image: %s (size: %s)", path, size)

	// Create sparse file with truncate
	cmd := exec.CommandContext(ctx, "truncate", "-s", size, path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("truncate failed: %w\n%s", err, output)
	}

	// Format as ext4
	mkfsPath := b.findMkfsExt4()
	cmd = exec.CommandContext(ctx, mkfsPath, "-F", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.ext4 failed: %w\n%s", err, output)
	}

	return nil
}

func (b *Builder) mount(ctx context.Context, source, target string) error {
	b.logger.Infof("Mounting %s to %s", source, target)
	cmd := exec.CommandContext(ctx, "sudo", "mount", source, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mount failed: %w\n%s", err, output)
	}
	return nil
}

func (b *Builder) unmount(target string) error {
	cmd := exec.Command("sudo", "umount", target)
	cmd.Run() // Ignore errors - might not be mounted
	return nil
}

func (b *Builder) exportDocker(ctx context.Context, imageName, mountPoint string) error {
	b.logger.Infof("Exporting Docker image %s to %s", imageName, mountPoint)

	// Create container
	createCmd := exec.CommandContext(ctx, "docker", "create", imageName)
	output, err := createCmd.Output()
	if err != nil {
		return fmt.Errorf("docker create failed: %w", err)
	}
	containerID := strings.TrimSpace(string(output))
	defer func() {
		exec.Command("docker", "rm", containerID).Run()
	}()

	b.logger.Infof("Created container %s", containerID)

	// Export and extract
	// Use: docker export <container> | sudo tar -xf - -C <mountpoint>
	exportCmd := exec.CommandContext(ctx, "docker", "export", containerID)
	tarCmd := exec.CommandContext(ctx, "sudo", "tar", "-xf", "-", "-C", mountPoint)

	// Connect pipes
	exportStdout, err := exportCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get docker export stdout: %w", err)
	}
	tarCmd.Stdin = exportStdout
	tarCmd.Stderr = os.Stderr

	// Start both commands
	if err := exportCmd.Start(); err != nil {
		return fmt.Errorf("docker export failed to start: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		exportCmd.Process.Kill()
		return fmt.Errorf("tar failed to start: %w", err)
	}

	// Wait for both
	exportErr := exportCmd.Wait()
	tarErr := tarCmd.Wait()

	if exportErr != nil {
		return fmt.Errorf("docker export failed: %w", exportErr)
	}
	if tarErr != nil {
		return fmt.Errorf("tar extract failed: %w", tarErr)
	}

	return nil
}

func (b *Builder) injectInitScript(mountPoint, script string) error {
	initPath := filepath.Join(mountPoint, "sbin", "init")
	b.logger.Infof("Injecting init script to %s", initPath)

	// Ensure /sbin exists
	sbinDir := filepath.Join(mountPoint, "sbin")
	if err := os.MkdirAll(sbinDir, 0755); err != nil {
		// Try with sudo
		cmd := exec.Command("sudo", "mkdir", "-p", sbinDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create /sbin: %w", err)
		}
	}

	// Write init script using sudo tee
	cmd := exec.Command("sudo", "tee", initPath)
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to write init script: %w\n%s", err, output)
	}

	// Make executable
	cmd = exec.Command("sudo", "chmod", "+x", initPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to chmod init script: %w\n%s", err, output)
	}

	return nil
}

func (b *Builder) ensureUtilities(ctx context.Context, imageName, mountPoint string) error {
	// Check if 'ip' command exists
	ipPaths := []string{
		filepath.Join(mountPoint, "sbin", "ip"),
		filepath.Join(mountPoint, "usr", "sbin", "ip"),
		filepath.Join(mountPoint, "bin", "ip"),
		filepath.Join(mountPoint, "usr", "bin", "ip"),
	}

	for _, p := range ipPaths {
		if _, err := os.Stat(p); err == nil {
			return nil // Found it
		}
	}

	b.logger.Warn("'ip' command not found in rootfs, attempting to install iproute2...")

	// Try to install via docker run
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", mountPoint+":/mnt",
		imageName,
		"bash", "-c", "apt-get update && apt-get install -y iproute2 && cp $(which ip) /mnt/sbin/ 2>/dev/null || true")
	cmd.Run() // Best effort

	return nil
}

func (b *Builder) moveOutput(src, dst string) error {
	b.logger.Infof("Moving %s to %s", src, dst)

	// Ensure output directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		// Try with sudo
		cmd := exec.Command("sudo", "mkdir", "-p", dstDir)
		cmd.Run()
	}

	// Try regular move first
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + remove (for cross-filesystem moves)
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		// Try with sudo
		cmd := exec.Command("sudo", "cp", src, dst)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to copy: %w\n%s", err, output)
		}
		return nil
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}
