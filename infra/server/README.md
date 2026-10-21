# Server configuration pack

Use [the CD runbook](../../docs/cd-pipeline.md) for the exact server/GitHub setup
and deployment actions. Nothing here installs or deploys automatically.

- `bootstrap.sh`: one-time Linux deployment-user/directory/config preparation.
- `deploy.json.example`: server-local paths, topology, HTTPS verification and hook.
- `sshd-mohdeploy.conf.example`: optional SSH restrictions for the dedicated user.
- `hooks/backup-primary.sh.example`: optional tested primary-DB backup hook template.

Application credentials remain on the server in `/etc/moh-sso`; never add real
values to these examples or GitHub build artifacts. The CD bundle contains only
public deployment infrastructure and immutable image metadata.
