using System.Diagnostics;
using ClaudeFF.Core;
namespace ClaudeFF.Windows;
public static class RelayManager
{
    public static string SessionFile(string id)
    {
        if(!Guid.TryParseExact(id,"N",out _))throw new ArgumentException("无效代理会话。");
        return Path.Combine(LocalStore.Root,"relay-"+id+".json");
    }
    public static RelaySession Start(Settings settings, Action<string> recordSession)
    {
        _=NetworkProbe.Socks(settings.ProxyHost,settings.ProxyPort,settings.ProxyUser,LocalStore.Unprotect(settings.ProtectedPassword));
        var session=new RelaySession{ProxyHost=settings.ProxyHost,ProxyPort=settings.ProxyPort,ProxyUser=settings.ProxyUser,ProtectedPassword=settings.ProtectedPassword};
        var file=SessionFile(session.Id);recordSession(session.Id);Json.Write(file,session);
        var info=new ProcessStartInfo(Path.Combine(AppContext.BaseDirectory,ExecutableNames.Relay)){UseShellExecute=false,CreateNoWindow=true};info.ArgumentList.Add("--session");info.ArgumentList.Add(file);
        using var process=Process.Start(info)??throw new InvalidOperationException("无法启动 TCP 代理后台进程。");
        for(int i=0;i<100;i++)
        {
            if(File.Exists(file+".ready"))
            {
                var ready=Json.Read<RelaySession>(file+".ready");
                if(ready.Id!=session.Id||ready.Pid!=process.Id||ready.Port is <1 or >65535)throw new InvalidDataException("代理启动响应无效。");
                return ready;
            }
            if(process.HasExited)throw new InvalidOperationException("代理进程启动失败，请检查凭证与程序文件。");
            Thread.Sleep(100);
        }
        File.WriteAllText(file+".stop","");throw new TimeoutException("代理进程启动超时。");
    }
    public static void Stop(string id)
    {
        var file=SessionFile(id);File.WriteAllText(file+".stop","");
    }
}
