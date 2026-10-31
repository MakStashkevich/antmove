class Antmove < Formula
  desc "Move Mac apps and data to external drives while keeping their paths working"
  homepage "https://github.com/MakStashkevich/antmove"
  url "https://github.com/MakStashkevich/antmove/archive/6e3505c0cc46bdc00d97f21f5aae2ebdfb4845fe.tar.gz"
  version "0.1.0"
  sha256 "9244bd0eb083eb5466d4d3953e710c52356c78519eb395444ee4c7dbb52d23dd"
  license "MIT"

  depends_on macos: :sequoia
  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags", "-s -w -X main.version=#{version}", "-o", bin/"antmove", "."
    bin.install_symlink "antmove" => "am"
  end
end
