#!/usr/bin/env python3

import os


def re_template(filepath):
    # Read lines, substitute, write back

    print(f"File {filepath}")
    lines = list(open(filepath).readlines())
    print(f"Processing {len(lines)} lines")
    labels = {}
    in_labels = False
    i_labels_start = 0
    i_labels_end = 0

    for i, l in enumerate(lines):
        if l.strip() == "labels:":
            l_indent = l.count(" ", 0, l.find("l"))
            in_labels = True
            i_labels_start = i
            print(f"Found labels at {i}")
            print(f"Labels indent is {l_indent}")
            continue
        if in_labels:
            if l.startswith((2 + l_indent) * " "):
                k, v = l.strip().split(":")
                print(f"Found label {k}={v}")
                labels[k.strip()] = v.strip()
            else:

                # Inject custom labels
                app_name = labels.get("app.kubernetes.io/name", "")
                if app_name != "":
                    labels["app"] = app_name

                labels_s = " ".join([f'"{k}" "{v}"' for k, v in labels.items()])
                labels_template = (
                    "  {{- "
                    + f'include "helm_lib_module_labels" (list . (dict {labels_s}) | nindent {l_indent}'
                    + " }}\n"
                )
                print(f"Labels end at {i}")
                lines[i_labels_start] = labels_template
                i_labels_end = i
                for j in range(i_labels_start + 1, i_labels_end):
                    lines[j] = ""
                labels = {}
                in_labels = False
            continue

        if l.strip().startswith("image: "):
            image_parts = lines[i].split(": ")
            lines[i] = image_parts[0] + ": " + map_image(image_parts[1].strip()) + "\n"

    with open(filepath, "w") as f:
        f.writelines([l for l in lines if l.strip() != ""])


def map_image(image):
    if image.find("updater") > -1:
        return '{{ include "helm_lib_module_image" (list . "argocdImageUpdater") }}'
    if image.find("argo") > -1:
        return '{{ include "helm_lib_module_image" (list . "argocd") }}'
    if image.find("redis") > -1:
        return '{{ include "helm_lib_module_image" (list . "redis") }}'
    return "UNKNOWN"


if __name__ == "__main__":
    for root, dirs, files in os.walk(os.path.join(os.getcwd(), "templates", "argocd")):
        print(root)
        # print("\n".join(["  ./" + d for d in dirs]))

        argo_files = [f for f in files if f.endswith("yaml")]
        print("\n".join(["    " + f for f in argo_files]))

        for f in argo_files:
            filepath = os.path.join(root, f)
            re_template(filepath)
