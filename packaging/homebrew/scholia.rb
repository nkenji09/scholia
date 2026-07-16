# Homebrew formula for scholia.
#
# This file is a hand-authored draft. Once GoReleaser's `brews:` publisher
# is wired up (DESIGN §10-2), it will regenerate this formula on each
# release and push it to the tap repo, filling in the real `url`/`sha256`
# per platform. Until then, every `url`/`sha256` below is a PLACEHOLDER —
# do not `brew install` from this file as-is.
class Scholia < Formula
  desc "AI 向けコンテキスト保存支援 CLI (scholia)"
  homepage "https://github.com/nkenji09/scholia"
  version "0.0.0-dev"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/nkenji09/scholia/releases/download/v#{version}/scholia_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
    else
      url "https://github.com/nkenji09/scholia/releases/download/v#{version}/scholia_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/nkenji09/scholia/releases/download/v#{version}/scholia_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_ARM64"
    else
      url "https://github.com/nkenji09/scholia/releases/download/v#{version}/scholia_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install "scholia"
  end

  test do
    system "#{bin}/scholia", "--help"
  end
end
