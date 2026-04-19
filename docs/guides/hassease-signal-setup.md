# Hassease Signal Setup

Signal-native tool approval for the Hassease daemon. The agent sends
"I want to edit X" over Signal, you reply y/n from your phone.

## Prerequisites

- Java 21+ (signal-cli is a Java application)
- A spare phone number for the Hassease account (VoIP numbers usually work)
- Your phone number as the builder/recipient

## 1. Install Java

```bash
# Ubuntu/Debian
sudo apt install openjdk-21-jre-headless

# Or via SDKMAN
curl -s "https://get.sdkman.io" | bash
sdk install java 21.0.4-tem
```

## 2. Install signal-cli

```bash
SIGNAL_CLI_VERSION=0.13.12
curl -L "https://github.com/AsamK/signal-cli/releases/download/v${SIGNAL_CLI_VERSION}/signal-cli-${SIGNAL_CLI_VERSION}-Linux.tar.gz" \
  | sudo tar xzf - -C /opt

sudo ln -sf /opt/signal-cli-${SIGNAL_CLI_VERSION}/bin/signal-cli /usr/local/bin/signal-cli
signal-cli --version
```

## 3. Register the Hassease account

```bash
# Request verification code (use --voice for a phone call instead of SMS)
signal-cli -a +HASSEASE_NUMBER register

# Complete registration with the code you received
signal-cli -a +HASSEASE_NUMBER verify CODE

# Verify it works
signal-cli -a +HASSEASE_NUMBER send -m "hassease online" +YOUR_NUMBER
```

## 4. Configure Hassease

```bash
cd os/Skaffen/cmd/hassease
cp hassease.example.yaml hassease.yaml

# Edit signal section:
#   account: "+HASSEASE_NUMBER"
#   recipient: "+YOUR_NUMBER"
```

## 5. Smoke test

```bash
# From os/Skaffen/
./scripts/hassease-signal-smoke.sh hassease.yaml
```

This sends a test message, waits for your reply, and reports success/failure.

## Troubleshooting

**"signal-cli not found"**: Ensure it's on PATH or set `signal.binary` in config.

**"no reply within 5m0s"**: Check your phone — the message may be in a
message request folder if you haven't messaged the Hassease number before.
Accept the message request, then retry.

**Java version errors**: signal-cli 0.13+ requires Java 21. Check with
`java -version`.

**Rate limiting on register**: Signal rate-limits registration. Wait and retry,
or use `--captcha` with a captcha token from https://signalcaptchas.org/registration/generate.html.
