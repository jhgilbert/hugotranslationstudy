# hugotranslationstudy

## Important disclaimer

My Go skills are _extremely_ questionable. This was just for fun!

## What is this package?

This package explores methods for

- Converting Hugo content files to JSON
- Translating that JSON (in this case, just to Piglatin)
- Assembling the translated .md content file

As a bonus quest, it also explores

- Converting a Hugo .md file to an .mdoc file

## Browse the results

Each file in the [content directory](./content/) has a corresponding folder in the [out directory](./out/), containing these files:

- `tokens.txt`: A printout of the tokens parsed from the file, just for learning/debugging purposes.
- `data.json`: The file as data that could be sent to a translator.
- `translated.md`: The content file in Piglatin.
- `migrated.mdoc`: The file migrated to Markdoc, replacing Hugo shortcodes with Markdoc tags.

For example, [this simple test file](./content/01_simple.md) generated [this output folder](./out/01_simple/).
