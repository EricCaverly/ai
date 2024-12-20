## Overview
This is going to become a custom Neuro-Sama clone. I do not take credit for the idea, however this is my implementation.

## Getting Started
1. Clone the repo `git clone https://github.com/EricCaverly/ai.git`
2. `cd` into the directory, `cd ai`
3. Enter your discord bot API token with the following command `echo "@TOKENHERE" >> discord/token.hide`
4. Build the local images with `./build_images.sh` (This will take a while)
5. Bring the stack up with `docker compose up -d`

## TODO

- Improve discord  implementation
    - Determine userid for SSRC (who is talking in voice call)
    - Speaking
    - Responding to messages
- Implement conversational language model
- Implemnet text-to-speech engine
- Add website functionality
    - Status indecators
    - Settings and controls