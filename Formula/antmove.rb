class Antmove < Formula
  desc "Move Mac apps and data to external drives while keeping their paths working"
  homepage "https://github.com/MakStashkevich/antmove"
  url "https://github.com/MakStashkevich/antmove/archive/refs/tags/v0.1.4.tar.gz"
  version "0.1.4"
  sha256 "1b7344f3c063075c0fdc4924b477c57b2f63720ea8d6ff0f427fcb93379a283f"
  license "MIT"

  depends_on macos: :sequoia
  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags", "-s -w -X main.version=#{version}", "-o", bin/"antmove", "."
    bin.install_symlink "antmove" => "am"
  end
end
