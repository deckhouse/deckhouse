#!/usr/bin/env python3

import base64
import datetime
import os.path
import re
import textwrap
import io

from cryptography import x509


def convert_certdata_text(certdata_text: str) -> None:
    objects = []

    in_data = False
    in_multiline = False
    in_obj = False
    field = None
    type_ = None
    value = None
    obj = dict()

    for line in io.StringIO(certdata_text):
        if not in_data:
            if line.startswith("BEGINDATA"):
                in_data = True
            continue

        if line.startswith("#"):
            continue

        if in_obj and len(line.strip()) == 0:
            objects.append(obj)
            obj = dict()
            in_obj = False
            continue

        if len(line.strip()) == 0:
            continue

        if in_multiline:
            if not line.startswith("END"):
                if type_ == "MULTILINE_OCTAL":
                    line = line.strip()
                    for i in re.finditer(r"\\([0-3][0-7][0-7])", line):
                        value.append(int(i.group(1), 8))
                else:
                    value += line
                continue

            obj[field] = value
            in_multiline = False
            continue

        if line.startswith("CKA_CLASS"):
            in_obj = True

        line_parts = line.strip().split(" ", 2)

        if len(line_parts) > 2:
            field, type_ = line_parts[0:2]
            value = " ".join(line_parts[2:])
        elif len(line_parts) == 2:
            field, type_ = line_parts
            value = None
        else:
            raise NotImplementedError("line_parts < 2 not supported.")

        if type_ == "MULTILINE_OCTAL":
            in_multiline = True
            value = bytearray()
            continue

        obj[field] = value

    if len(obj) > 0:
        objects.append(obj)

    blacklist = []

    if os.path.exists("blacklist.txt"):
        for line in open("blacklist.txt", "r"):
            line = line.strip()
            if line.startswith("#") or len(line) == 0:
                continue
            item = line.split("#", 1)[0].strip()
            blacklist.append(item)

    trust = dict()

    for obj in objects:
        if obj["CKA_CLASS"] != "CKO_NSS_TRUST":
            continue

        if obj["CKA_LABEL"] in blacklist:
            pass
        elif obj["CKA_TRUST_SERVER_AUTH"] == "CKT_NSS_TRUSTED_DELEGATOR":
            trust[obj["CKA_LABEL"]] = True
        elif obj["CKA_TRUST_SERVER_AUTH"] == "CKT_NSS_NOT_TRUSTED":
            pass
        else:
            _ = obj["CKA_LABEL"]
            _ = obj["CKA_TRUST_SERVER_AUTH"]
            _ = obj["CKA_TRUST_EMAIL_PROTECTION"]

    for obj in objects:
        if obj["CKA_CLASS"] != "CKO_CERTIFICATE":
            continue

        if obj["CKA_LABEL"] not in trust or not trust[obj["CKA_LABEL"]]:
            continue

        cert = x509.load_der_x509_certificate(bytes(obj["CKA_VALUE"]))

        if cert.not_valid_after < datetime.datetime.utcnow():
            pass

        bname = (
            obj["CKA_LABEL"][1:-1]
            .replace("/", "_")
            .replace(" ", "_")
            .replace("(", "=")
            .replace(")", "=")
            .replace(",", "_")
        )

        bname = bname.encode("utf-8").decode("unicode_escape").encode("latin-1")

        fname = bname + b".crt"

        if os.path.exists(fname):
            fname = bname + b"_2.crt"

        with open(fname, "w") as f:
            f.write("-----BEGIN CERTIFICATE-----\n")
            encoded = base64.b64encode(obj["CKA_VALUE"]).decode("utf-8")
            f.write("\n".join(textwrap.wrap(encoded, 64)))
            f.write("\n-----END CERTIFICATE-----\n")
