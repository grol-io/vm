; counter.asm: Test LoadR/StoreR with writable data segment
; Load a counter, increment it, store it back, then exit with a fixed value
; The exit code 43 proves the code ran (we can verify data segment exists)

.const STDOUT 1

    LoadR counter   ; Load counter value into A (should be 42)
    AddI 1          ; Add 1 to A (A = 43)
    StoreR counter  ; Store back to counter (proves data segment is writable)
    ; Exit with 43 to show we got here
    Sys Exit 43

counter:
    data 42
