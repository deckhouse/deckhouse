#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This script partially fills manifests with deckhouse-specific Helm function calls. Resulting
# manifests are only partially templated and are not completely correct. Manual revision must follow
# before commiting.
#
# Usage: call it from the module root:
#
#   ./hack/retemplate.py
#
import os


class Aggregator:
    def __init__(self):
        self.overrides = []  # (i, line)
        self.erasures = []  # i
        self.insertions = []  # (i, line)

    def substitute(self, i: int, line: str):
        self.overrides.append((i, line))

    def erase(self, i: int):
        self.erasures.append(i)

    def insert(self, i: int, line: str):
        self.insertions.append((i, line))

    def apply(self, lines: list):
        """
        Modifies lines in-place.
        """

        for i, line in self.overrides:
            print(f"Overriding line {i}: {line}")
            lines[i] = line

        for i in self.erasures:
            print(f"Erasing line {i}: {lines[i]}")
            lines[i] = ""

        shift = 0
        self.insertions.sort()
        for i, line in self.insertions:
            print(f"Inserting line {i+shift}: {line}")
            lines.insert(i + shift, line)
            shift += 1


class Replacer:
    def __init__(self, aggregator: Aggregator):
        self.aggregator = aggregator


class ImageReplacer(Replacer):
    def process(self, i: int, line: str):
        if line.strip().startswith("image: "):
            delim = ": "
            image_parts = line.split(delim)
            print(f"Found image '{image_parts[1]}'")
            image = self._image_template(image_parts[1].strip())
            new_line = image_parts[0] + delim + image
            self.aggregator.substitute(i, new_line)

    def _image_template(self, image: str):
        """
        Match image name by substring and return Helm template function call.
        """
        if image.find("updater") > -1:
            return '{{ include "helm_lib_module_image" (list . "argocdImageUpdater") }}'
        if image.find("argo") > -1:
            return '{{ include "helm_lib_module_image" (list . "argocd") }}'
        if image.find("redis") > -1:
            return '{{ include "helm_lib_module_image" (list . "redis") }}'
        return "UNKNOWN"


class ImagePullPolicyReplacer(Replacer):
    def process(self, i: int, line: str):
        if line.strip().startswith("imagePullPolicy: "):
            delim = ": "
            parts = line.split(delim)
            new_line = parts[0] + delim + "IfNotPresent"
            self.aggregator.substitute(i, new_line)


class LabelReplacer(Replacer):
    def __init__(self, aggregator: Aggregator):
        super().__init__(aggregator)
        self.in_labels = False
        self.start = 0
        self.nindent = 0
        self.labels = {}

    def reset(self):
        self.in_labels = False
        self.start = 0
        self.nindent = 0
        self.labels = {}

    def process(self, i: int, line: str):
        if line.strip() == "labels:":
            self.nindent = line.count(" ", 0, line.find("l"))
            self.in_labels = True
            self.start = i
            print(f"Found labels at {i}")
            print(f"Labels nindent is {self.nindent}")
            return

        if not self.in_labels:
            return

        # Collect existing labels
        kv_indent = (2 + self.nindent) * " "
        if line.startswith(kv_indent):
            k, v = line.strip().split(":", maxsplit=1)
            k, v = k.strip(), v.strip()
            print(f"Found label {k}={v} (indent={2 + self.nindent})")
            self.labels[k] = v
            return

        # After labels, form the transformations
        app_name = self.labels.get("app.kubernetes.io/name", "")
        print(f"Labels end at {i-1}")
        if self.nindent > 2:
            # This is not top-level labels. Pods should have only additional 'app' label. We are
            # just adding it.
            label_line = f"{kv_indent}app: {app_name}\n"
            self.aggregator.insert(i, label_line)
        else:
            # Inject custom labels
            if app_name != "":
                self.labels["app"] = app_name
            if "dummy" in self.labels:
                # this is from a hack to have labels in YAML
                del self.labels["dummy"]

            new_line = self._template(self.labels, self.nindent)
            self.aggregator.substitute(self.start, new_line)

            for j in range(self.start + 1, i):
                self.aggregator.erase(j)

        self.reset()

    def _template(self, labels: dict, nindent: int):
        """
        Get module labels Helm template function call.
        """
        kv_str = " ".join([f'"{k}" "{v}"' for k, v in labels.items()])
        return " ".join(
            (
                " " * (nindent - 1),  # Helm template indentation
                "{{-",
                f'include "helm_lib_module_labels" (list . (dict {kv_str}))',
                f"| nindent {nindent}",
                "}}\n",
            )
        )


class SecurityContextReplacer(Replacer):
    def __init__(self, aggregator: Aggregator):
        super().__init__(aggregator)
        self.in_seccontext = False
        self.nindent = 0

    def reset(self):
        self.in_seccontext = False
        self.nindent = 0

    def process(self, i, line):
        # Substitute securityContext with *drop_all
        if line.strip() == "securityContext:":
            print(f"Found securityContext at {i}")
            self.in_seccontext = True
            self.nindent = line.count(" ", 0, line.find("s"))
            new_line = self._template(self.nindent)
            self.aggregator.substitute(i, new_line)
            return

        if self.in_seccontext:
            # drop all inner lines
            if line.startswith((2 + self.nindent) * " "):
                self.aggregator.erase(i)
            else:
                self.reset()

    def _template(self, nindent):
        """
        Get common security context for containers and Pods
        """
        return " ".join(
            (
                " " * (nindent - 1),  # Helm template indentation
                "{{-",
                'include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" .',
                f"| nindent {nindent}",
                "}}\n",
            )
        )


def re_template(filepath: str):
    print(f"File {filepath}")
    lines = list(open(filepath, encoding="utf-8").readlines())

    agg = Aggregator()
    replacers = [
        LabelReplacer(agg),
        SecurityContextReplacer(agg),
        ImageReplacer(agg),
        ImagePullPolicyReplacer(agg),
    ]

    for i, line in enumerate(lines):
        for replacer in replacers:
            replacer.process(i, line)

    agg.apply(lines)

    new_lines = []
    for line in lines:
        if line.strip() == "":
            continue
        if not line.endswith("\n"):
            line = line + "\n"
        new_lines.append(line)

    with open(filepath, "w", encoding="utf-8") as f:
        f.writelines(new_lines)


def main():
    """
    Walk through Argo CD templates and insert Helm template functions.
    """

    templates_root = os.path.join(os.getcwd(), "templates", "argocd")
    for root, _, files in os.walk(templates_root):
        print(root)

        argo_files = [f for f in files if f.endswith("yaml")]
        print("\n".join(["    " + f for f in argo_files]))

        for f in argo_files:
            filepath = os.path.join(root, f)
            re_template(filepath)


if __name__ == "__main__":
    main()
