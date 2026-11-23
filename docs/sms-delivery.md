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

The test SMS API returns `503 SMS_DISABLED`, and the SMS delivery worker is not started, so any
SMS deliveries that are queued stay `PENDING`.

### Mock

```env
SMS_ENABLED=true
SMS_PROVIDER=mock
```

Use mock for local development. The provider logs delivery attempts and returns a provider
message ID without contacting an external SMS gateway.

`SMS_PROVIDER=noop` is also accepted; it fails every send with `sms provider is disabled`, so
deliveries retry and then become `FAILED`.

### Africa's Talking

```env
SMS_ENABLED=true
SMS_PROVIDER=africastalking
AFRICASTALKING_USERNAME=...
AFRICASTALKING_API_KEY=...
AFRICASTALKING_SENDER_ID=...
```

`AFRICASTALKING_USERNAME` and `AFRICASTALKING_API_KEY` are required when SMS is enabled with this
provider; startup validation fails without them. The sender ID is optional: when it is empty no
`from` value is sent, so the account's default sender is used.

### Twilio

Twilio configuration is reserved in the runtime config:

```env
SMS_PROVIDER=twilio
TWILIO_ACCOUNT_SID=...
TWILIO_AUTH_TOKEN=...
TWILIO_FROM_NUMBER=...
```

All three values are required by startup validation when SMS is enabled with this provider, but
no Twilio adapter exists: `SMS_PROVIDER=twilio` uses the mock provider, which only logs the message
and marks the delivery `SENT` with provider `twilio` (`backend/internal/service/sms_service.go`).
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

These are the defaults in `backend/internal/config/config.go` and in `infra/env/dev.env.example`
and `infra/env/prod.env.example`. Phone numbers are normalised with `SMS_DEFAULT_COUNTRY_CODE`
(a leading `0` is replaced by the country code) and must have 8 to 15 digits. `SMS_MAX_LENGTH`
counts characters. `SMS_FROM_NAME` fills an empty message sender, but neither the mock nor the
Africa's Talking provider uses it (Africa's Talking sends `AFRICASTALKING_SENDER_ID`). Each provider
call is bounded by `SMS_SEND_TIMEOUT`.

The SMS worker uses the shared `NOTIFICATION_DELIVERY_WORKER_ENABLED`,
`NOTIFICATION_DELIVERY_WORKER_INTERVAL` (default `3s`) and `NOTIFICATION_DELIVERY_BATCH_SIZE`
(default `20`) settings.

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

The endpoint requires `notifications:write` and returns `202` with the notification ID and status
`PENDING`. It queues an SMS delivery (`max_attempts` 3) and records an in-app delivery, which is
`SENT` as soon as it is stored. The admin dashboard
exposes this as **Send test SMS**.

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
- `sms recipient phone is required` or `phone number must be 8 to 15 digits` (delivery last error):
  check the user preference phone number or test SMS input.
- `sms body exceeds max length of N characters`: reduce message length or raise `SMS_MAX_LENGTH`
  intentionally.
- Delivery stuck in `PENDING`: check `NOTIFICATION_DELIVERY_WORKER_ENABLED`, `SMS_ENABLED` and
  backend logs.
- Delivery stuck in `PROCESSING`: the SMS worker reclaims it (or fails it, if that was its last attempt) once its 15-minute lease expires; if it stays longer, check that the backend and SMS worker are running, or cancel it from the admin dashboard.
- Provider failure: inspect provider metadata and backend logs, then retry after fixing credentials/network issues.
