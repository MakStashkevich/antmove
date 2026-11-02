class Antmove < Formula
  desc "Move Mac apps and data to external drives while keeping their paths working"
  homepage "https://github.com/MakStashkevich/antmove"
  url "https://github.com/MakStashkevich/antmove/archive/refs/tags/v0.1.3.tar.gz"
  version "0.1.3"
  sha256 "ed820979eaec47dae36e0fd383b98b731d82c6a0da6afbab698fa24002818b5e"
  license "MIT"

  depends_on macos: :sequoia
  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags", "-s -w -X main.version=#{version}", "-o", bin/"antmove", "."
    bin.install_symlink "antmove" => "am"
  end
end
