package buildinfo

// These values are replaced with -ldflags for release builds. Development
// binaries keep explicit local values rather than pretending to be a release.
var (
	Version       = "dev"
	GitSHA        = "unknown"
	BuildTime     = "unknown"
	ImageDigest   = "unknown"
	ReleaseID     = "local"
	SchemaVersion = "unknown"
)

func Public() map[string]string {
	return map[string]string{
		"version": Version, "git_sha": GitSHA, "build_time": BuildTime,
		"image_digest": ImageDigest, "release_id": ReleaseID, "schema_version": SchemaVersion,
	}
}
