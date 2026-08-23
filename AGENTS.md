# Engineering Governance

Before making any change in this repository:

1. Initialize the pinned `.engineering-public` submodule.
2. Read `.engineering-public/README.md` and every public policy document it
   lists.
3. If `.engineering-private/AGENTS.md` exists, read it and every private
   document it requires before applying repository-specific instructions.

The public rules always apply. A local `.engineering-private` overlay is
optional only for ordinary public-source work. Internal, release,
infrastructure, migration, secret, or production work must not proceed when
that overlay is unavailable or unread. This file is a discovery adapter and
does not create or override policy.
