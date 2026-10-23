#!/usr/bin/env python3
"""Prepare private portal/Health BI scheduler settings without contacting Keycloak."""
import argparse
import os
from pathlib import Path
import re
import secrets
import tempfile
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[2]


def values(path):
    result = {}
    if not path.exists():
        return result
    for line in path.read_text().splitlines():
        key, separator, value = line.partition('=')
        if separator and re.fullmatch(r'[A-Z][A-Z0-9_]*', key.strip()):
            value = value.strip()
            if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
                quote = value[0]
                value = value[1:-1].replace('\\' + quote, quote)
            result[key.strip()] = value
    return result


def encoded(value):
    if any(char in value for char in '\r\n\0'):
        raise ValueError('Environment values must be single-line strings')
    if not value or re.fullmatch(r'[A-Za-z0-9_./:@+-]+', value):
        return value
    # Single quotes keep dollar signs literal in Compose dotenv files.
    return "'" + value.replace("'", "\\'") + "'"


def update(path, settings, template=None):
    source = path if path.exists() else template
    lines = source.read_text().splitlines() if source else []
    output, written = [], set()
    for line in lines:
        key = line.partition('=')[0].strip()
        if key not in settings:
            output.append(line)
        elif key not in written:
            output.append(key + '=' + encoded(settings[key]))
            written.add(key)
    output.extend(key + '=' + encoded(value) for key, value in settings.items() if key not in written)
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix='.scheduler-', dir=path.parent)
    try:
        with os.fdopen(descriptor, 'w') as stream:
            os.fchmod(stream.fileno(), 0o600)
            stream.write('\n'.join(output) + '\n')
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def base_url(value):
    parsed = urlsplit(value)
    if (parsed.scheme not in ('http', 'https') or not parsed.hostname or parsed.username or parsed.password
            or parsed.query or parsed.fragment or any(c.isspace() for c in value)
            or parsed.path.rstrip('/').endswith('/api/v1')):
        raise ValueError('Use an HTTP(S) Health BI application base without credentials, query strings or /api/v1')
    parsed.port  # validate malformed ports before changing any files
    return value.rstrip('/')


def generate(portal, healthbi, mode, url='', public_url='', client_secret_file=None, template=None):
    if portal.is_symlink() or healthbi.is_symlink() or portal.resolve() == healthbi.resolve():
        raise ValueError('Portal and Health BI env files must be distinct regular paths')
    if portal.name.endswith('.example') or healthbi.name.endswith('.example'):
        raise ValueError('Use private env files, not checked-in examples')
    if portal.exists() and not portal.is_file():
        raise ValueError('The application env destination must be a regular file')
    if not portal.exists() and (template is None or not template.is_file()):
        raise ValueError(f'Missing application env file: {portal}. Copy the corresponding infra/env/*.env.example to this path first, or omit ENV_FILE to create the default automatically.')
    current, peer = values(portal if portal.exists() else template), values(healthbi)
    defaults = values(ROOT / 'infra/env/prod.env.example')
    default_base = 'http://host.docker.internal:8080' if mode == 'dev' else defaults['HEALTH_BI_BASE_URL']
    application_url = base_url(url or current.get('HEALTH_BI_BASE_URL') or default_base)
    public_base = public_url or peer.get('SCHEDULER_PUBLIC_BASE_URL')
    if not public_base:
        public_base = 'http://localhost:8080' if urlsplit(application_url).hostname == 'host.docker.internal' else application_url
    public_base = base_url(public_base)
    issuer = base_url(current.get('KEYCLOAK_EXTERNAL_URL', ''))
    realm = current.get('KEYCLOAK_REALM', '')
    if not realm:
        raise ValueError('The application env file must configure KEYCLOAK_REALM')
    client_id = current.get('HEALTH_BI_KEYCLOAK_CLIENT_ID') or 'report-scheduler-worker'
    portal_settings = {
        'HEALTH_BI_BASE_URL': application_url,
        'HEALTH_BI_KEYCLOAK_CLIENT_ID': client_id,
        'HEALTH_BI_TIMEOUT': current.get('HEALTH_BI_TIMEOUT') or '30s',
        'REPORT_SCHEDULER_WORKER_INTERVAL': current.get('REPORT_SCHEDULER_WORKER_INTERVAL') or '30s',
        'REPORT_SCHEDULER_DELIVERY_LINK_TTL': current.get('REPORT_SCHEDULER_DELIVERY_LINK_TTL') or '24h',
    }
    # Do not rewrite or synthesize a Keycloak-issued credential.
    credential_present = bool(current.get('HEALTH_BI_KEYCLOAK_CLIENT_SECRET') or current.get('HEALTH_BI_KEYCLOAK_CLIENT_SECRET_FILE'))
    if client_secret_file:
        credential = Path(client_secret_file).read_text().rstrip('\r\n')
        if not credential:
            raise ValueError('The Keycloak worker client secret file is empty')
        portal_settings['HEALTH_BI_KEYCLOAK_CLIENT_SECRET'] = credential
        if current.get('HEALTH_BI_KEYCLOAK_CLIENT_SECRET_FILE'):
            portal_settings['HEALTH_BI_KEYCLOAK_CLIENT_SECRET_FILE'] = ''
        credential_present = True
    elif 'HEALTH_BI_KEYCLOAK_CLIENT_SECRET' not in current:
        portal_settings['HEALTH_BI_KEYCLOAK_CLIENT_SECRET'] = ''
    signing_key = peer.get('SCHEDULER_ARTIFACT_SIGNING_KEY') or peer.get('SCHEDULER_SERVICE_API_KEY') or secrets.token_urlsafe(48)
    if len(signing_key) < 32:
        raise ValueError('The existing Health BI signing key must be at least 32 characters; it was not rotated')
    healthbi_settings = {
        'KEYCLOAK_URL': issuer,
        'KEYCLOAK_REALM': realm,
        'BASE_PATH': urlsplit(public_base).path.rstrip('/'),
        'SCHEDULER_KEYCLOAK_CLIENT_ID': client_id,
        'SCHEDULER_KEYCLOAK_AUDIENCE': 'data-statistics',
        'SCHEDULER_KEYCLOAK_ROLE': 'report_scheduler_service',
        'SCHEDULER_ARTIFACT_SIGNING_KEY': signing_key,
        'SCHEDULER_PUBLIC_BASE_URL': public_base,
    }
    # Validate all generated values before writing either file.
    for value in [*portal_settings.values(), *healthbi_settings.values()]:
        encoded(value)
    update(portal, portal_settings, template)
    update(healthbi, healthbi_settings)
    return credential_present


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=['dev', 'external', 'prod', 'gateway', 'remote'], default=os.environ.get('SCHEDULER_MODE') or 'external')
    parser.add_argument('--env-file', default=os.environ.get('SCHEDULER_ENV_FILE') or None)
    parser.add_argument('--healthbi-env-file', default=os.environ.get('HEALTHBI_ENV_FILE') or None)
    parser.add_argument('--health-bi-url', default=os.environ.get('HEALTH_BI_URL', ''))
    parser.add_argument('--public-url', default=os.environ.get('HEALTH_BI_PUBLIC_URL', ''))
    parser.add_argument('--client-secret-file', default=os.environ.get('SCHEDULER_CLIENT_SECRET_FILE') or None)
    args = parser.parse_args()
    profile = 'prod' if args.mode in ('prod', 'gateway', 'remote') else args.mode
    name = 'external-dev' if profile == 'external' else profile
    portal = Path(args.env_file) if args.env_file else ROOT / f'infra/env/{name}.env'
    healthbi = Path(args.healthbi_env_file) if args.healthbi_env_file else ROOT / f'infra/env/scheduler-healthbi.{profile}.env'
    if not portal.is_absolute():
        portal = ROOT / portal
    if not healthbi.is_absolute():
        healthbi = ROOT / healthbi
    template = ROOT / f'infra/env/{name}.env.example' if not args.env_file else None
    creating = not portal.exists()
    configured = generate(portal, healthbi, profile, args.health_bi_url, args.public_url, args.client_secret_file, template)
    if creating:
        print('Created private application env from the mode template; fill remaining deployment credentials/settings before starting services.')
    print('Portal scheduler settings: ' + str(portal))
    print('Health BI scheduler settings (install separately): ' + str(healthbi))
    print('Signing key retained/generated privately; Keycloak was not modified.')
    if not configured:
        print('Next: supply the report-scheduler-worker client secret in HEALTH_BI_KEYCLOAK_CLIENT_SECRET; the integration remains disabled until it is set.')


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError) as error:
        raise SystemExit(str(error))
