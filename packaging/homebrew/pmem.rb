# Homebrew formula for pmem.
#
# This file is a hand-authored draft. Once GoReleaser's `brews:` publisher
# is wired up (DESIGN §10-2), it will regenerate this formula on each
# release and push it to the tap repo, filling in the real `url`/`sha256`
# per platform. Until then, every `url`/`sha256` below is a PLACEHOLDER —
# do not `brew install` from this file as-is.
class Pmem < Formula
  desc "AI 向けコンテキスト保存支援 CLI (product-memory)"
  homepage "https://github.com/nkenji09/product-memory"
  version "0.0.0-dev"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/nkenji09/product-memory/releases/download/v#{version}/pmem_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
    else
      url "https://github.com/nkenji09/product-memory/releases/download/v#{version}/pmem_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/nkenji09/product-memory/releases/download/v#{version}/pmem_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_ARM64"
    else
      url "https://github.com/nkenji09/product-memory/releases/download/v#{version}/pmem_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install "pmem"
  end

  test do
    system "#{bin}/pmem", "--help"
  end
end
