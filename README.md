LRCLIB Fetcher
==============

A Go terminal UI tool that fetches lyrics from LRCLIB for music files using their tags. It saves synced lyrics as .lrc and plain lyrics as .txt. Ambiguous search results can be reviewed and selected manually.

Usage
-----
Build and run from the project directory:

    go build -o lrclib-fetcher .
    ./lrclib-fetcher

Choose a music folder in the UI, adjust the options, and run the fetcher. FLAC files are scanned by default; MP3 scanning can be enabled. Existing lyric files can be skipped.

Options
-------
- Fetch jobs: number of concurrent LRCLIB requests.
- Jobs: number of workers for file processing and writing.
- Max retries: retry limit for API requests.
- Verbose / Debug: control log detail.
- Skip existing .lrc / .txt: avoid processing tracks that already have those lyric files.

Lyrics are written beside the source audio file. Internet access and a reachable LRCLIB API are required.
