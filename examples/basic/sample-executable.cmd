@Echo Off
ChCp 65001 >Nul

Echo 0 (stdout) ...

TimeOut /T 2

Echo 1 (stderr) ... 1>&2
Echo 2 (stderr) ... 1>&2
Echo 3 (stdout) ...

TimeOut /T 2

Echo 4 (stdout) ...
Echo 5 (stderr) ... 1>&2
Echo 6 (stdout) ...
Echo 7 (stderr) ... 1>&2
Echo 8 (stdout) ...
Echo 9 (stderr) ... 1>&2

Echo %CD%
Echo Arg: %1

Pause
Exit /B 0
