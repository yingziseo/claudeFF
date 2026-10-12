using System.Globalization;
using System.Runtime.InteropServices;
using System.Text;
using ClaudeFF.Core;
using Microsoft.Win32;

if(!OperatingSystem.IsWindows()) return 2;
if(args.Length!=2 || args[0]!="--request") return 2;
try
{
    var request=Json.Read<ProbeRequest>(args[1]);
    if(request.Nonce.Length!=32 || !Path.IsPathFullyQualified(request.Output)) return 2;
    var result=new ProbeResult { Nonce=request.Nonce,Pid=Environment.ProcessId,Box=Native.Box(),Locale=CultureInfo.CurrentCulture.Name };
    foreach(var file in request.Files) result.Access.Add(ReadFile(file));
    foreach(var directory in request.Directories) result.Access.Add(ReadDirectory(directory));
    try { using var key=Registry.CurrentUser.OpenSubKey(request.RegistryKey);result.RegistryOutcome=key?.GetValue("Value") is string ? "readable" : "missing"; }
    catch(UnauthorizedAccessException){result.RegistryOutcome="denied";}
    catch(System.Security.SecurityException){result.RegistryOutcome="denied";}
    catch { result.RegistryOutcome="error"; }
    foreach(var target in request.RegistryTargets)
    {
        try { using var key=Registry.CurrentUser.OpenSubKey(target.Path);result.RegistryAccess.Add(new(target.Label,target.Path,key!=null?"readable":"missing")); }
        catch(UnauthorizedAccessException e){result.RegistryAccess.Add(new(target.Label,target.Path,"denied",e.HResult));}
        catch(System.Security.SecurityException e){result.RegistryAccess.Add(new(target.Label,target.Path,"denied",e.HResult));}
        catch(Exception e){result.RegistryAccess.Add(new(target.Label,target.Path,"error",e.HResult));}
    }
    if(request.RelayChallenge)
    {
        try
        {
            using var tcp=new System.Net.Sockets.TcpClient();using var routeDeadline=new CancellationTokenSource(TimeSpan.FromSeconds(6));
            await tcp.ConnectAsync(SocksBridge.CanaryAddress,SocksBridge.CanaryPort,routeDeadline.Token);
            await tcp.GetStream().WriteAsync(Encoding.ASCII.GetBytes(request.Nonce),routeDeadline.Token);
            result.RelayResponse=Encoding.ASCII.GetString(await SocksBridge.Read(tcp.GetStream(),47,routeDeadline.Token));
        }
        catch { result.RelayResponse=null; }
    }
    if(request.Network)
    {
        try { result.ExitIp=await NetworkProbe.ExitIpAsync(null,false,CancellationToken.None); }
        catch(Exception ex) { result.NetworkError=ex.GetType().Name; }
    }
    Json.Write(request.Output,result);
    // Remain alive so the verifier can query THIS process via the driver, not trust our JSON claim.
    var deadline=DateTime.UtcNow.AddSeconds(30);
    while(DateTime.UtcNow<deadline && !File.Exists(request.Output+".ack")) await Task.Delay(100);
    return 0;
}
catch { return 3; }

static AccessSample ReadFile(Target target)
{
    try { using var stream=File.Open(target.Path,FileMode.Open,FileAccess.Read,FileShare.ReadWrite|FileShare.Delete);_ = stream.ReadByte();return new(target.Label,target.Path,"readable"); }
    catch(UnauthorizedAccessException e){return new(target.Label,target.Path,"denied",e.HResult);}
    catch(FileNotFoundException e){return new(target.Label,target.Path,"missing",e.HResult);}
    catch(DirectoryNotFoundException e){return new(target.Label,target.Path,"missing",e.HResult);}
    catch(Exception e){return new(target.Label,target.Path,"error",e.HResult);}
}
static AccessSample ReadDirectory(Target target)
{
    try { using var enumerator=Directory.EnumerateFileSystemEntries(target.Path).GetEnumerator();_ = enumerator.MoveNext();return new(target.Label,target.Path,"readable"); }
    catch(UnauthorizedAccessException e){return new(target.Label,target.Path,"denied",e.HResult);}
    catch(DirectoryNotFoundException e){return new(target.Label,target.Path,"missing",e.HResult);}
    catch(Exception e){return new(target.Label,target.Path,"error",e.HResult);}
}
static class Native
{
    [DllImport("kernel32.dll",CharSet=CharSet.Unicode)] private static extern IntPtr GetModuleHandle(string name);
    [DllImport("kernel32.dll",CharSet=CharSet.Ansi,ExactSpelling=true)] private static extern IntPtr GetProcAddress(IntPtr module,string name);
    [UnmanagedFunctionPointer(CallingConvention.StdCall,CharSet=CharSet.Unicode)] private delegate int Query(IntPtr pid,StringBuilder box,IntPtr image,IntPtr sid,IntPtr session);
    public static string? Box()
    {
        var module=GetModuleHandle("SbieDll.dll");if(module==IntPtr.Zero)return null;
        var entry=GetProcAddress(module,"SbieApi_QueryProcess");if(entry==IntPtr.Zero)return null;
        var box=new StringBuilder(34);return Marshal.GetDelegateForFunctionPointer<Query>(entry)(new(Environment.ProcessId),box,IntPtr.Zero,IntPtr.Zero,IntPtr.Zero)==0?box.ToString():null;
    }
}
