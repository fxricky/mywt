# mywt Homebrew tap formula
#
# This file lives in a separate tap repo: github.com/fxricky/homebrew-tap
# at Formula/mywt.rb. Users install with:
#
#   brew install fxricky/tap/mywt
#
# It builds from source (Homebrew installs Go temporarily), so no prebuilt
# binaries are required. On each release, run scripts/release.sh <version>
# from the mywt repo to update the url + sha256 below.

class Mywt < Formula
  desc "Manage git worktrees for many projects in one place"
  homepage "https://github.com/fxricky/mywt"
  url "https://github.com/fxricky/mywt/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"

  # macOS only for now.
  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X github.com/fxricky/mywt/internal/version.version=v#{version}")
  end

  test do
    assert_match "v#{version}", shell_output("#{bin}/mywt version")
  end
end
