# Video-agent brief: LazyConteiner v1.0.0

## Assignment

Create a clear, confident 60–75 second product introduction for LazyConteiner.
Use the supplied real screenshots and application icon. Show what the product
helps a Debian Docker user do, how to install it, and where to get it. Do not
invent features or manufacture user testimonials. Read `README.md` in this media
kit and the repository's feature limitations before writing final captions.

The desired result is an understandable product demonstration, not a rapid
montage of unreadable screens. Keep the tone practical, lightly playful and
respectful of terminal users. The tagline is humorous; the product still expects
users to understand Docker permissions and destructive actions.

## Deliverables

- 1920×1080, 16:9 master, 60–75 seconds, MP4/H.264, 30 fps.
- A 20–30 second 1080×1920 teaser, with composition adapted for vertical viewing.
- English narration script and editable subtitles (SRT).
- One thumbnail, an editable project/source file, and a list of music/font licenses.
- End card linking to `github.com/AxiomaAbsurdo/LazyConteiner`.

Keep narration audible over music. Use readable captions, adequate contrast and
motion that does not compete with the UI. Do not add sound effects for operations
that were never recorded. Include the release version and Debian 13 amd64 target.

## Storyboard and draft narration

| Time | Visual | Draft narration / screen copy |
| --- | --- | --- |
| 0–7 s | Supplied icon, then hero dashboard | “A GUI for containers—for those days when you’re not feeling nerdy enough for the terminal.” |
| 7–16 s | `01-dashboard.png`; gentle move toward inventory | “Meet LazyConteiner: a native desktop app for managing local Docker on Debian.” |
| 16–28 s | `02-containers.png`; highlight inventory and details | “Find a container, check its status, and get to the details you need from one workspace.” |
| 28–39 s | `04-stacks.png`, then `03-images.png` | “Work with Compose projects, images, volumes and networks alongside your containers.” |
| 39–49 s | Container action area; additional recorded preview only if available | “Review exact targets before running an action. Keep your native terminal available for interactive work.” |
| 49–59 s | `05-narrow.png`, then a simple installation card | “Install the Debian package. No development toolchain required. Explore demo mode before connecting to your Docker environment.” |
| 59–70 s | Icon, repository URL, version and download call to action | “Try LazyConteiner for Debian 13 amd64. Download it on GitHub, contribute, or support the project.” |

The times are a pacing guide, not a requirement to stretch stills unnaturally.
Adjust narration after timing the actual read. Use “Docker” and “Compose” as
product names; preserve “LazyConteiner” in written text.

## Installation card

Display these commands legibly, one step at a time:

```sh
sudo apt install ./lazyconteiner_1.0.0-1_amd64.deb
lazyconteiner --demo
```

Point viewers to the release page for the download and checksum. Explain in a
caption that live mode uses their existing Docker installation. Do not imply the
package installs Docker or grants access to its socket.

## Optional additional screen recording

If interactive footage is needed, use the actual app in isolated demo mode:

1. Launch `lazyconteiner --demo` with the normal desktop user.
2. Pause on the dashboard, then navigate to Containers.
3. Type a filter and show the matching inventory.
4. Select a demo resource, open an action preview, and cancel it.
5. Navigate to Images and Compose projects.
6. Resize the window and show scrolling without obscuring labels.
7. Close the app normally.

Use the same camera scale across related scenes. Do not animate a pointer as if it
clicked successfully when only a still screenshot is available. Do not record
real containers or run destructive Docker commands to make a promotional shot.

## Acceptance checklist

- Product name, tagline, target OS and release version are correct.
- Screens are real, legible and labeled as demo data when shown.
- No unsupported platform, backend or embedded-terminal claims.
- No fabricated benchmark, customer endorsement or human-test sign-off.
- Download and optional PayPal URLs match the product kit.
- Subtitles match the final voiceover; music and fonts have usable licenses.
- Final output ends with a clear GitHub download call to action.
