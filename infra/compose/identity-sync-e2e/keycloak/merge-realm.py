"""Merge infra/keycloak/realm-export.json with the e2e overlay.

Usage: python3 merge-realm.py <base.json> <overlay.json> <out.json>

Rules (kept deliberately small):
- clients are matched by clientId and users by username; an overlay entry is
  deep-merged into the base entry (overlay values win, lists are replaced),
  and unmatched overlay entries are appended;
- clientScopeMappings are merged per client, the overlay list replacing the base one;
- every other top-level key in the overlay replaces the base value.
Keys starting with "_" in the overlay are comments and are dropped.
"""
import json
import sys


def strip_comments(value):
    if isinstance(value, dict):
        return {k: strip_comments(v) for k, v in value.items() if not k.startswith("_")}
    if isinstance(value, list):
        return [strip_comments(v) for v in value]
    return value


def deep_merge(base, overlay):
    out = dict(base)
    for key, value in overlay.items():
        if isinstance(value, dict) and isinstance(out.get(key), dict):
            out[key] = deep_merge(out[key], value)
        else:
            out[key] = value
    return out


def merge_list(base, overlay, key):
    index = {item[key]: i for i, item in enumerate(base)}
    out = list(base)
    for item in overlay:
        if item[key] in index:
            out[index[item[key]]] = deep_merge(out[index[item[key]]], item)
        else:
            index[item[key]] = len(out)
            out.append(item)
    return out


def main(base_path, overlay_path, out_path):
    with open(base_path) as f:
        realm = json.load(f)
    with open(overlay_path) as f:
        overlay = strip_comments(json.load(f))
    for key, value in overlay.items():
        if key == "clients":
            realm["clients"] = merge_list(realm.get("clients", []), value, "clientId")
        elif key == "users":
            realm["users"] = merge_list(realm.get("users", []), value, "username")
        elif key == "clientScopeMappings":
            realm.setdefault("clientScopeMappings", {}).update(value)
        elif key == "roles":
            for scope, roles in value.items():
                if scope == "client":
                    for client, client_roles in roles.items():
                        realm["roles"]["client"][client] = merge_list(
                            realm["roles"]["client"].get(client, []), client_roles, "name")
                else:
                    realm["roles"][scope] = merge_list(realm["roles"].get(scope, []), roles, "name")
        else:
            realm[key] = value
    with open(out_path, "w") as f:
        json.dump(realm, f, indent=1)
    print(f"merged realm {realm['realm']}: {len(realm['clients'])} clients, {len(realm['users'])} users -> {out_path}")


if __name__ == "__main__":
    if len(sys.argv) != 4:
        sys.exit(__doc__)
    main(*sys.argv[1:])
