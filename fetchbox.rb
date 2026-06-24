class Fetchbox < Formula
  desc "IMAP attachment fetcher that uploads to WebDAV (Nextcloud)"
  homepage "https://github.com/jchonig/docker-fetchbox"
  license "MIT"
  version "0.0.0" # replaced by update script

  on_macos do
    on_arm do
      url "https://github.com/jchonig/docker-fetchbox/releases/download/v#{version}/fetchbox-darwin-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # replaced by update script
    end
    on_intel do
      url "https://github.com/jchonig/docker-fetchbox/releases/download/v#{version}/fetchbox-darwin-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # replaced by update script
    end
  end

  def install
    bin.install "fetchbox"
  end

  service do
    run [opt_bin/"fetchbox", "--daemon", "--config", etc/"fetchbox/fetchbox.yml"]
    keep_alive true
    log_path var/"log/fetchbox.log"
    error_log_path var/"log/fetchbox.log"
  end

  def caveats
    <<~EOS
      Configuration: ~/.config/fetchbox.yml
      Credentials are stored in the macOS Keychain.

      Run once interactively to populate the Keychain, then start the service:
        fetchbox --list-folders
        brew services start fetchbox

      Or install the launchd agent manually:
        fetchbox --install
    EOS
  end

  test do
    assert_match "Usage", shell_output("#{bin}/fetchbox --help 2>&1", 2)
  end
end
