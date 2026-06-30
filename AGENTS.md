# Repository Notes for Agents

- When rebuilding the Windows GUI executable, include the GUI subsystem flag so double-click launch does not open a terminal window:

  ```powershell
  go build -buildvcs=false -ldflags "-H=windowsgui" -o bdp-sync.exe ./cmd/bdp-sync
  ```

- Use a plain `go build -o bdp-sync.exe ./cmd/bdp-sync` only when intentionally producing a console build.
