## How do I convert a temporary session into a regular project?

Temporary sessions are stored under `/tmp/spilltea/<random-id>/` and will be lost on reboot. To keep the data, move the directory into the Spilltea projects folder and give it a name.

**1. Find the temporary session directory**

```bash
ls /tmp/spilltea/
```

You will see one or more directories with a random hex name (e.g. `a1b2c3d4`).

**2. Move it to the projects folder**

```bash
mv /tmp/spilltea/<random-id> ~/.local/share/spilltea/<project-name>
```

The project name must only contain lowercase letters, digits, `-`, and `_`.

**3. Open the project**

```bash
spilltea -P my-project
```

Or simply launch Spilltea and pick `my-project` from the project list.
