"""Read-only by default. Sending a test message requires --recipient and --key."""
import argparse
import os
import sys
import time
from platform_client import PlatformClient, PlatformError, discover, machine_token


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--recipient")
    parser.add_argument("--key", help="stable key reused for retries of this test message")
    parser.add_argument("--poll-seconds", type=int, default=0)
    args = parser.parse_args()
    if args.recipient and not args.key:
        parser.error("--recipient requires --key")
    if not 0 <= args.poll_seconds <= 300:
        parser.error("--poll-seconds must be between 0 and 300")
    try:
        metadata = discover(os.environ["OIDC_ISSUER"])
        token = machine_token(metadata["token_endpoint"], os.environ["MACHINE_CLIENT_ID"], os.environ["MACHINE_CLIENT_SECRET"],
                              scope="identity:read notifications:read notifications:write" if args.recipient else "identity:read")
        client = PlatformClient(os.environ["PLATFORM_BASE_URL"])
        identity = client.identity(token)
        expected = os.environ["APPLICATION_ID"]
        if identity.get("applicationId") != expected:
            raise PlatformError("service account is bound to a different application")
        print("Identity and application binding verified for " + expected)
        if args.recipient:
            notification = client.notify(token, args.key, args.recipient,
                                         "Platform integration test", "The pilot application can use shared notifications.")
            deadline = time.monotonic() + args.poll_seconds
            while True:
                status = client.status(token, notification["id"])
                print("Notification %s: %s" % (status["id"], status["status"]))
                if status["status"] in ("FAILED", "CANCELLED", "EXPIRED"):
                    raise PlatformError("notification did not complete successfully")
                if status["status"] == "SENT" or time.monotonic() >= deadline:
                    break
                time.sleep(2)
            if args.poll_seconds and status["status"] != "SENT":
                raise PlatformError("notification is still pending after the polling deadline")
    except KeyError as error:
        print("Missing configuration: " + error.args[0], file=sys.stderr)
        return 1
    except PlatformError as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
