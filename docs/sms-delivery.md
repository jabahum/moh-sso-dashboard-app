# SMS delivery

SMS is implemented as a notification delivery channel. Application features should create
notifications with an `sms` delivery request instead of calling an SMS provider directly. This page is for developers
and operators who configure SMS providers or queue SMS from features; see [Notifications](./notifications.md) for the
wider delivery model.

## User preferences

SMS delivery requires:

- user notification preferences with `sms_enabled=true`
- a valid phone number
- a feature flow that queues an SMS delivery

Announcement SMS uses the announcement audience resolver first, then filters recipients by SMS
preference and phone availability.

## Announcement SMS workflow

1. Admin creates or edits an announcement.
2. Admin enables SMS notification and optionally writes an SMS-specific message.
3. On publish or due scheduled publish, the backend queues notification deliveries.
4. The SMS delivery worker claims `PENDING` or `RETRY` SMS deliveries.
5. The worker sends through the configured provider.
6. The delivery becomes `SENT`, `RETRY`, or `FAILED`.

## Providers

### Disabled

```env
SMS_ENABLED=false
```

SMS queueing APIs reject test SMS requests, and the SMS delivery worker is not started.

### Mock

```env
SMS_ENABLED=true
SMS_PROVIDER=mock
```

Use mock for local development. The provider logs delivery attempts and returns a provider
message ID without contacting an external SMS gateway.

### Africa's Talking

```env
SMS_ENABLED=true
SMS_PROVIDER=africastalking
AFRICASTALKING_USERNAME=...
AFRICASTALKING_API_KEY=...
AFRICASTALKING_SENDER_ID=...
```

The sender ID is optional when the provider account is configured to use a default sender.

### Twilio

Twilio configuration is reserved in the runtime config:

```env
SMS_PROVIDER=twilio
TWILIO_ACCOUNT_SID=...
TWILIO_AUTH_TOKEN=...
TWILIO_FROM_NUMBER=...
```

Do not enable Twilio in production until a Twilio provider adapter is completed and verified.

## Environment variables

```env
SMS_ENABLED=false
SMS_PROVIDER=mock
SMS_DEFAULT_COUNTRY_CODE=+256
SMS_FROM_NAME=MOH
SMS_MAX_LENGTH=160
SMS_SEND_TIMEOUT=10s

AFRICASTALKING_USERNAME=
AFRICASTALKING_API_KEY=
AFRICASTALKING_SENDER_ID=

TWILIO_ACCOUNT_SID=
TWILIO_AUTH_TOKEN=
TWILIO_FROM_NUMBER=
```

## Admin test SMS

Use:

```http
POST /api/v1/admin/notifications/test-sms
```

Body:

```json
{
  "to": "+256700000000",
  "message": "Test SMS"
}
```

The admin dashboard exposes this as **Send test SMS**.

## Production checklist

- Confirm `SMS_ENABLED=true`.
- Confirm the provider credentials are present in the runtime secret store.
- Confirm `SMS_MAX_LENGTH` matches provider and regulatory requirements.
- Confirm `NOTIFICATION_DELIVERY_WORKER_ENABLED=true`.
- Send a test SMS from the admin dashboard.
- Confirm delivery status transitions to `SENT`.
- Confirm audit logs record the test queue and worker outcome.
- Monitor failed/retry delivery counts.

## Troubleshooting

- `SMS_DISABLED`: enable SMS and restart the backend.
- `sms recipient is required`: check user preference phone number or test SMS input.
- `sms body exceeds max length`: reduce message length or raise `SMS_MAX_LENGTH` intentionally.
- Delivery stuck in `PENDING`: check worker enabled flag and backend logs.
- Delivery stuck in `PROCESSING`: check whether the backend crashed during processing; retry or cancel from the admin dashboard.
- Provider failure: inspect provider metadata and backend logs, then retry after fixing credentials/network issues.
