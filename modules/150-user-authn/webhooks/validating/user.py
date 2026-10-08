# Copyright 2024 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap


def validate(ctx: DotMap) -> tuple:
    operation = ctx.review.request.operation
    if operation == "CREATE" or operation == "UPDATE":
        return validate_creation_or_update(ctx)
    elif operation == "DELETE":
        return validate_delete(ctx)
    else:
        raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap) -> tuple:
    operation = ctx.review.request.operation
    user_name = ctx.review.request.object.metadata.name
    user_id = ctx.review.request.object.spec.userID
    email = ctx.review.request.object.spec.email
    groups = ctx.review.request.object.spec.groups

    # Email validation with backward compatibility
    has_upper = bool(email) and email != email.lower()
    old_email = None
    email_changed = False
    old_has_upper = False

    if operation == "UPDATE" and ctx.review.request.oldObject is not None:
        old_email = ctx.review.request.oldObject.spec.email
        email_changed = old_email is not None and old_email != email
        old_has_upper = bool(old_email) and old_email != old_email.lower()

    # Case-insensitive email uniqueness check
    if operation == "CREATE" or (operation == "UPDATE" and email_changed):
        user_with_the_same_email = [
            obj.filterResult for obj in ctx.snapshots.users
            if obj.filterResult.name != user_name and obj.filterResult.email and obj.filterResult.email.lower() == email.lower()
        ]
        if user_with_the_same_email:
            return f"users.deckhouse.io \"{user_name}\", user \"{user_with_the_same_email[0].name}\" is already using email \"{user_with_the_same_email[0].email}\" (case-insensitive match)", False

    # CREATE: forbid uppercase emails
    if operation == "CREATE" and has_upper:
        return f"users.deckhouse.io \"{user_name}\", \".spec.email\" must be lowercase. Use \"{email.lower()}\" instead", False

    # UPDATE: forbid changing lowercase email to uppercase
    # Exception: if the old email already contained uppercase, allow updates to preserve backward compatibility
    if operation == "UPDATE" and email_changed and has_upper and not old_has_upper:
        return f"users.deckhouse.io \"{user_name}\", changing \".spec.email\" to contain uppercase is forbidden; use lowercase", False

    # Legacy updates: if old email had uppercase, allow updates; warn if the resulting email still has uppercase
    if operation == "UPDATE" and old_has_upper:
        if has_upper:
            return "\".spec.email\" contains uppercase; Dex lowercases emails. Consider migrating to lowercase.", True

    # Original email uniqueness check (exact match) - keep for backward compatibility
    user_with_the_same_email = [obj.filterResult for obj in ctx.snapshots.users if obj.filterResult.name != user_name and obj.filterResult.email == email]
    if user_with_the_same_email:
        return f"users.deckhouse.io \"{user_name}\", user \"{user_with_the_same_email[0].name}\" is already using email \"{email}\"", False

    if operation == "CREATE" and groups:
        return "\".spec.groups\" is deprecated, use the \"Group\" object.", False

    if operation == "UPDATE" and groups:
        snapshot_user = next((user.filterResult for user in ctx.snapshots.users if user.filterResult.name == user_name), None)
        if snapshot_user and set(snapshot_user.groups) - set(groups):
            return "\".spec.groups\" is deprecated, modification is forbidden, only removal of all elements is allowed", False

    if email.startswith("system:"):
        return f"users.deckhouse.io \"{user_name}\", \".spec.email\" must not start with the \"system:\" prefix", False

    if user_id:
        return "\".spec.userID\" is deprecated and shouldn't be set manually (if set, its value is ignored)", True

    return None, True


def validate_delete(ctx: DotMap) -> tuple:
    user_name = ctx.review.request.oldObject.metadata.name
    warnings = []

    for group in ctx.snapshots.groups:
        for member in group.filterResult.members:
            if member.kind == "User" and member.name == user_name:
                warnings.append(f"groups.deckhouse.io \"{group.filterResult.name}\" contains users.deckhouse.io \"{user_name}\"")

    # The template takes a single warning string, and the API server refuses a warning with a
    # line break in it, so one warning per group is joined into one line.
    return "; ".join(warnings), True
