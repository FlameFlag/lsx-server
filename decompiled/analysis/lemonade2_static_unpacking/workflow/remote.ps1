param(
  [string]$WorkDir = "C:\Users\Admin\AppData\Local\Temp\lemonade2_static_unpacking",
  [string]$GameExe = "C:\Program Files (x86)\Lemonade Tycoon 2 - New York City\Lemonade2.exe",
  [int]$RunSeconds = 60,
  [string]$GameArgs = "",
  [switch]$AutoRegister,
  [string]$RegistrationName = "TestName",
  [string]$ActivationKey = "0000PP-FZYKGQ-JABWAK-Q6XMT6-U0U72Q-CD4Y50-JTAV0G",
  [switch]$DumpMemory,
  [switch]$DataGuard,
  [switch]$DisableSeedHook,
  [switch]$BuildOnly
)

$ErrorActionPreference = "Stop"

$trace = "C:\Users\Admin\AppData\Local\Temp\lemonade2_api_trace"
$out = Join-Path $WorkDir "out"
$dll = Join-Path $WorkDir "hook.dll"
$launcher = Join-Path $WorkDir "launcher.exe"
$log = Join-Path $out "capture.log"
$earlyDumpDir = Join-Path $out "mem_dump_early"
$lateDumpDir = Join-Path $out "mem_dump_late"
$zip = Join-Path $WorkDir "capture.zip"

New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null
Remove-Item $trace -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $out -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $zip -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $trace | Out-Null
New-Item -ItemType Directory -Force -Path $out | Out-Null

function Log-Line([string]$line) {
  $line | Tee-Object -FilePath $log -Append
}

function Build-Binaries() {
  Log-Line "BUILD hook"
  i686-w64-mingw32-gcc -shared -O2 -Wall -Wextra -o $dll (Join-Path $WorkDir "hook.c") -lkernel32 -luser32 2>&1 | Tee-Object -FilePath $log -Append

  Log-Line "BUILD launcher"
  i686-w64-mingw32-gcc -O2 -Wall -Wextra -o $launcher (Join-Path $WorkDir "launcher.c") -lkernel32 2>&1 | Tee-Object -FilePath $log -Append
}

function Invoke-AutoRegister([string]$Name, [string]$Key) {
  $src = @"
using System;
using System.Runtime.InteropServices;
public static class User32AutoReg {
  [DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Ansi)] public static extern IntPtr FindWindowA(string cls, string title);
  [DllImport("user32.dll", SetLastError=true)] public static extern IntPtr GetDlgItem(IntPtr hwnd, int id);
  [DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Ansi)] public static extern bool SetWindowTextA(IntPtr hwnd, string text);
  [DllImport("user32.dll", SetLastError=true)] public static extern IntPtr SendMessageA(IntPtr hwnd, UInt32 msg, IntPtr wparam, IntPtr lparam);
}
"@
  if (-not ([System.Management.Automation.PSTypeName]'User32AutoReg').Type) {
    Add-Type $src
  }
  $WM_COMMAND = 0x0111
  $BN_CLICKED_OK = 1
  $deadline = (Get-Date).AddSeconds(25)
  while ((Get-Date) -lt $deadline) {
    $hwnd = [User32AutoReg]::FindWindowA($null, "Enter Key")
    if ($hwnd -ne [IntPtr]::Zero) {
      $nameEdit = [User32AutoReg]::GetDlgItem($hwnd, 1031)
      $keyEdit = [User32AutoReg]::GetDlgItem($hwnd, 1045)
      [User32AutoReg]::SetWindowTextA($nameEdit, $Name) | Out-Null
      [User32AutoReg]::SetWindowTextA($keyEdit, $Key) | Out-Null
      [User32AutoReg]::SendMessageA($hwnd, $WM_COMMAND, [IntPtr]$BN_CLICKED_OK, [IntPtr]::Zero) | Out-Null
      Log-Line "AUTO_REGISTER submitted"
      return
    }
    Start-Sleep -Milliseconds 250
  }
  Log-Line "AUTO_REGISTER dialog not found"
}

function Dump-ProcessMemory([string]$OutDir) {
  New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

  $src = @"
using System;
using System.Runtime.InteropServices;

public static class Win32Mem {
  [DllImport("kernel32.dll", SetLastError=true)]
  public static extern IntPtr OpenProcess(UInt32 access, bool inherit, UInt32 pid);
  [DllImport("kernel32.dll", SetLastError=true)]
  public static extern bool CloseHandle(IntPtr h);
  [DllImport("kernel32.dll", SetLastError=true)]
  public static extern UIntPtr VirtualQueryEx(IntPtr h, UIntPtr addr, out MEMORY_BASIC_INFORMATION32 mbi, UInt32 len);
  [DllImport("kernel32.dll", SetLastError=true)]
  public static extern bool ReadProcessMemory(IntPtr h, UIntPtr addr, byte[] buf, UInt32 size, out UIntPtr read);

  [StructLayout(LayoutKind.Sequential)]
  public struct MEMORY_BASIC_INFORMATION32 {
    public UInt32 BaseAddress;
    public UInt32 AllocationBase;
    public UInt32 AllocationProtect;
    public UInt32 RegionSize;
    public UInt32 State;
    public UInt32 Protect;
    public UInt32 Type;
  }
}
"@
  if (-not ([System.Management.Automation.PSTypeName]'Win32Mem').Type) {
    Add-Type $src
  }

  $PROCESS_QUERY_INFORMATION = 0x0400
  $PROCESS_VM_READ = 0x0010
  $MEM_COMMIT = 0x1000
  $MEM_PRIVATE = 0x20000
  $MEM_IMAGE = 0x1000000
  $PAGE_GUARD = 0x100
  $PAGE_NOACCESS = 0x01

  function Is-ReadableProtect([uint32]$p) {
    if (($p -band $PAGE_GUARD) -ne 0) { return $false }
    $base = $p -band 0xff
    if ($base -eq 0 -or $base -eq $PAGE_NOACCESS) { return $false }
    return $true
  }

  function Type-Name([uint32]$t) {
    if ($t -eq $MEM_IMAGE) { return "IMAGE" }
    if ($t -eq $MEM_PRIVATE) { return "PRIVATE" }
    return ("0x{0:X8}" -f $t)
  }

  function Sha256-Hex([byte[]]$bytes, [int]$count) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
      $slice = New-Object byte[] $count
      [Array]::Copy($bytes, $slice, $count)
      return ([BitConverter]::ToString($sha.ComputeHash($slice))).Replace("-", "").ToLowerInvariant()
    } finally {
      $sha.Dispose()
    }
  }

  $processIds = @(Get-Process Lemonade2 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Id)
  Write-Output ("PROCESS_IDS " + (($processIds | ForEach-Object { $_.ToString() }) -join ","))

  $interesting = @(0x00401000,0x00492000,0x0049C000,0x004D31B3,0x004D3888,0x004DC5C3,0x01077D34,0x010995DB,0x0109983F)
  $summary = New-Object System.Collections.Generic.List[string]
  $summary.Add("pid,base,size,state,protect,type,allocation_base,allocation_protect,contains_interesting,sha256,path")

  foreach ($procId in $processIds) {
    $procDir = Join-Path $OutDir ("pid_{0}" -f $procId)
    New-Item -ItemType Directory -Force -Path $procDir | Out-Null
    $h = [Win32Mem]::OpenProcess($PROCESS_QUERY_INFORMATION -bor $PROCESS_VM_READ, $false, [uint32]$procId)
    if ($h -eq [IntPtr]::Zero) {
      $summary.Add(("{0},OPEN_FAILED,,,,,,,," -f $procId))
      continue
    }
    try {
      $addr = [uint32]0
      while ($addr -lt 0x7fff0000) {
        $mbi = New-Object Win32Mem+MEMORY_BASIC_INFORMATION32
        $got = [Win32Mem]::VirtualQueryEx($h, [UIntPtr]$addr, [ref]$mbi, [uint32][Runtime.InteropServices.Marshal]::SizeOf($mbi))
        if ($got.ToUInt32() -eq 0) { break }
        $base = [uint32]$mbi.BaseAddress
        $size = [uint32]$mbi.RegionSize
        $end = [uint64]$base + [uint64]$size
        $contains = @($interesting | Where-Object { ([uint32]$_) -ge $base -and ([uint32]$_) -lt $end })
        $shouldDump = $false
        if ($mbi.State -eq $MEM_COMMIT -and (Is-ReadableProtect $mbi.Protect)) {
          if ($mbi.Type -eq $MEM_PRIVATE) { $shouldDump = $true }
          if ($contains.Count -gt 0) { $shouldDump = $true }
          if ($base -ge 0x00400000 -and $base -lt 0x00590000) { $shouldDump = $true }
        }
        $sha = ""
        $path = ""
        if ($shouldDump) {
          $buf = New-Object byte[] $size
          $n = [UIntPtr]::Zero
          if ([Win32Mem]::ReadProcessMemory($h, [UIntPtr]$base, $buf, $size, [ref]$n)) {
            $count = [int]$n.ToUInt32()
            if ($count -gt 0) {
              $name = "region_{0:X8}_{1:X8}_{2}_{3:X8}.bin" -f $base,$count,(Type-Name $mbi.Type),$mbi.Protect
              $path = Join-Path $procDir $name
              [IO.File]::WriteAllBytes($path, $buf[0..($count-1)])
              $sha = Sha256-Hex $buf $count
            }
          }
        }
        $summary.Add(("{0},{1:X8},{2:X8},{3:X8},{4:X8},{5},{6:X8},{7:X8},{8},{9},{10}" -f `
          $procId,$base,$size,$mbi.State,$mbi.Protect,(Type-Name $mbi.Type),$mbi.AllocationBase,$mbi.AllocationProtect,`
          (($contains | ForEach-Object { "0x{0:X8}" -f $_ }) -join "|"),$sha,$path))
        $next = $end
        if ($next -le $addr) { break }
        $addr = [uint32]$next
      }
    } finally {
      [Win32Mem]::CloseHandle($h) | Out-Null
    }
  }

  [IO.File]::WriteAllLines((Join-Path $OutDir "summary.csv"), $summary.ToArray())
}

Log-Line "WORKDIR $WorkDir"
Get-Process Lemonade2 -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500

Build-Binaries

if ($BuildOnly) {
  Log-Line "BUILD_ONLY done"
  Compress-Archive -Path (Join-Path $out "*") -DestinationPath $zip -Force
  Log-Line "ZIP $zip"
  Write-Output $zip
  exit 0
}

Log-Line "EXE $GameExe"
Get-Item $GameExe | Select-Object FullName, Length, LastWriteTime | Format-List | Out-String | Tee-Object -FilePath $log -Append | Out-Host

Log-Line "RUN launcher"
if ($DataGuard) {
  $env:LEMONADE2_DATA_GUARD = "1"
  Log-Line "DATA_GUARD enabled"
} else {
  Remove-Item Env:\LEMONADE2_DATA_GUARD -ErrorAction SilentlyContinue
}
if ($DisableSeedHook) {
  $env:LEMONADE2_DISABLE_SEED_HOOK = "1"
  Log-Line "SEED_HOOK disabled"
} else {
  Remove-Item Env:\LEMONADE2_DISABLE_SEED_HOOK -ErrorAction SilentlyContinue
}
& $launcher $GameExe $dll $GameArgs 2>&1 | Tee-Object -FilePath $log -Append

if ($AutoRegister) {
  Invoke-AutoRegister $RegistrationName $ActivationKey
}

if ($DumpMemory) {
  Log-Line "MEMORY DUMP early"
  Start-Sleep -Seconds 5
  Dump-ProcessMemory $earlyDumpDir 2>&1 | Tee-Object -FilePath $log -Append
  $remaining = $RunSeconds - 5
  if ($remaining -gt 0) { Start-Sleep -Seconds $remaining }
} else {
  Start-Sleep -Seconds $RunSeconds
}

Log-Line "PROCESSES"
Get-Process Lemonade2 -ErrorAction SilentlyContinue | Select-Object Id, ProcessName, MainWindowTitle | Format-Table -AutoSize | Out-String | Tee-Object -FilePath $log -Append | Out-Host

Log-Line "TRACE FILES"
Get-ChildItem $trace -ErrorAction SilentlyContinue | Sort-Object Name | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize | Out-String | Tee-Object -FilePath $log -Append | Out-Host

if (Test-Path (Join-Path $trace "trace.log")) {
  Log-Line "TRACE TAIL"
  Get-Content (Join-Path $trace "trace.log") -Tail 160 | Tee-Object -FilePath $log -Append
}

if ($DumpMemory) {
  Log-Line "MEMORY DUMP late"
  Dump-ProcessMemory $lateDumpDir 2>&1 | Tee-Object -FilePath $log -Append
}

Copy-Item $trace (Join-Path $out "trace") -Recurse -Force -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $out "*") -DestinationPath $zip -Force
Log-Line "ZIP $zip"
Write-Output $zip
