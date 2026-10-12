using System.Diagnostics;
using ClaudeFF.Core;
using Microsoft.Win32;
namespace ClaudeFF.Windows;
public static class Verifier
{
    public static async Task<Report> RunAsync(IProgress<string>? progress,CancellationToken token)
    {
        try { return await RunCoreAsync(progress,token); }
        catch(Exception error)
        {
            var failed=new Report {Checks=[new("execution","本次防护验证执行",Verdict.Unknown,"未能执行实测："+LocalStore.HidePath(error.Message),true)]};
            Json.Write(Path.Combine(LocalStore.Root,"latest-report.json"),failed);
            return failed;
        }
    }
    private static async Task<Report> RunCoreAsync(IProgress<string>? progress,CancellationToken token)
    {
        using var operationLock=PolicyManager.Lock();
        var report=new Report { Scope="仅检查本工具已启用的规则及本次实测路径。不是 Claude 功能测试，也不证明未列出的保护。" };
        var journal=PolicyManager.Current();
        if(journal is null || journal.State!="Applied")
        {
            report.Checks.AddRange(TelemetrySettings.Verify());
            Json.Write(Path.Combine(LocalStore.Root,"latest-report.json"),report);
            return report;
        }
        using var api=new SandboxApi(journal.SandboxHome);
        if(journal.Box!=LocalStore.Box)throw new InvalidOperationException("防护记录不属于当前用户。");
        var probe=Path.Combine(AppContext.BaseDirectory,ExecutableNames.Probe);
        if(!File.Exists(probe))throw new FileNotFoundException("后台验证探针缺失。请完整解压安装包，保留所有文件。");
        progress?.Report("正在核对已启用的访问规则…");
        foreach(var change in journal.Changes)
        {
            bool same=api.Read(journal.Box,change.Key).SequenceEqual(change.After);
            if(!same) report.Checks.Add(new("config-"+change.Key,"防护配置一致性",Verdict.Fail,"配置已变更："+change.Key+"。需重新应用与验证。",true));
        }
        var runDir=Path.Combine(LocalStore.Runs,report.RunId);Directory.CreateDirectory(runDir);
        var canary=Path.Combine(LocalStore.Canary,report.RunId+".txt");Directory.CreateDirectory(LocalStore.Canary);
        File.WriteAllText(canary,Guid.NewGuid().ToString("N"));
        var regPath=PolicyManager.RegistryCanary+"\\"+report.RunId;
        using(var key=Registry.CurrentUser.CreateSubKey(regPath))key.SetValue("Value",report.RunId);
        var nonce=Guid.NewGuid().ToString("N");
        var request=new ProbeRequest { Nonce=nonce,Box=journal.Box,Output=Path.Combine(runDir,"host.json"),Files=[new("受限测试文件",canary)],Directories=journal.Targets,RegistryKey=regPath,RegistryTargets=journal.RegistryTargets,Network=false };
        try
        {
            progress?.Report("正在建立宿主可访问基线…");
            var host=await RunProbe(api,probe,request,runDir,false,token);
            progress?.Report("正在沙箱内实际尝试访问…");
            request.Output=Path.Combine(runDir,"sandbox.json");
            request.RelayChallenge=journal.RelayId!=null;request.Network=journal.RelayId!=null;
            var sandbox=await RunProbe(api,probe,request,runDir,true,token);
            bool membership=sandbox.Box==journal.Box;
            report.Checks.Add(new("membership","沙箱接管",membership?Verdict.Pass:Verdict.Fail,"运行时进程归属："+(sandbox.Box??"未处于目标沙箱"),true));
            foreach(var sample in host.Access)
            {
                var actual=sandbox.Access.FirstOrDefault(a=>a.Path==sample.Path);
                var check=VerificationRules.FileDenied("access-"+report.Checks.Count,sample,membership?actual:null);
                report.Checks.Add(check with { Evidence=check.Evidence+" 路径："+LocalStore.HidePath(sample.Path) });
            }
            foreach(var sample in host.RegistryAccess)
            {
                var actual=sandbox.RegistryAccess.FirstOrDefault(a=>a.Path==sample.Path);
                var check=VerificationRules.FileDenied("browser-registry-"+report.Checks.Count,sample,membership?actual:null);
                report.Checks.Add(check with {Evidence=check.Evidence+" 键：HKCU\\"+sample.Path});
            }
            Verdict reg=host.RegistryOutcome!="readable" || !membership ? Verdict.Unknown : sandbox.RegistryOutcome=="denied" ? Verdict.Pass : sandbox.RegistryOutcome=="readable" ? Verdict.Fail : Verdict.Unknown;
            report.Checks.Add(new("registry","专属测试注册表访问限制",reg,$"宿主：{host.RegistryOutcome}；沙箱：{sandbox.RegistryOutcome}。仅代表本工具的测试键。",true));
            if(journal.RelayId!=null)
            {
                bool routed=membership && sandbox.RelayResponse=="claudeFF-relay:"+nonce;
                report.Checks.Add(new("tcp-route","已启用的 TCP 代理接管",routed?Verdict.Pass:Verdict.Fail,"沙箱 TCP 探针需通过本工具代理回显本次随机挑战；"+(routed?"回显匹配。":"未取得匹配回显。"),true));
                report.Checks.Add(new("tcp-exit","TCP 代理出口连通",sandbox.ExitIp is not null&&routed?Verdict.Pass:Verdict.Unknown,sandbox.ExitIp is {} ip?"沙箱测试请求的出口 IP："+ip:"未取得出口响应，不能证明代理可正常联网。",true));
            }
            // DNS, UDP, IPv6 and unimplemented protections are deliberately excluded.
        }
        catch(Exception error)
        {
            report.Checks.Add(new("verification-incomplete","已启用防护的实测执行",Verdict.Unknown,"实测未完成："+LocalStore.HidePath(error.Message),true));
        }
        finally
        {
            try{File.Delete(canary);}catch(IOException){}
            try{Registry.CurrentUser.DeleteSubKeyTree(regPath,false);}catch(UnauthorizedAccessException){}
            // Run folder contains no private file content. Keep only the redacted final report.
            try{Directory.Delete(runDir,true);}catch(IOException){}catch(UnauthorizedAccessException){}
        }
        try{report.Checks.AddRange(TelemetrySettings.Verify());}catch(Exception error){report.Checks.Add(new("settings-readback","已修改设置的状态读取",Verdict.Unknown,LocalStore.HidePath(error.Message),true));}
        Json.Write(Path.Combine(LocalStore.Root,"latest-report.json"),report);
        progress?.Report(report.Summary);
        return report;
    }
    private static async Task<ProbeResult> RunProbe(SandboxApi api,string probe,ProbeRequest request,string dir,bool boxed,CancellationToken token)
    {
        string requestFile=Path.Combine(dir,boxed?"sandbox-request.json":"host-request.json");Json.Write(requestFile,request);
        Process launcher;
        if(boxed) launcher=api.Start(request.Box,probe,"--request",requestFile);
        else
        {
            var start=new ProcessStartInfo(probe){UseShellExecute=false,CreateNoWindow=true};start.ArgumentList.Add("--request");start.ArgumentList.Add(requestFile);
            launcher=Process.Start(start)??throw new InvalidOperationException("探针启动失败。");
        }
        using(launcher)
        {
            try
            {
                using var timeout=CancellationTokenSource.CreateLinkedTokenSource(token);timeout.CancelAfter(TimeSpan.FromSeconds(60));
                while(!File.Exists(request.Output))
                {
                    if(launcher.HasExited && launcher.ExitCode!=0)throw new InvalidOperationException("探针启动失败，退出代码："+launcher.ExitCode);
                    timeout.Token.ThrowIfCancellationRequested();await Task.Delay(100,timeout.Token);
                }
                var result=Json.Read<ProbeResult>(request.Output);
                if(result.Nonce!=request.Nonce || result.Pid<=0)throw new InvalidDataException("探针响应不匹配，拒绝使用旧报告。");
                using var live=Process.GetProcessById(result.Pid);
                if(live.HasExited || !string.Equals(live.ProcessName,ExecutableNames.ProbeProcess,StringComparison.OrdinalIgnoreCase))throw new InvalidDataException("无法确认探针进程。");
                var driverBox=api.ProcessBox(result.Pid);
                if(boxed) result.Box=driverBox;
                else if(driverBox!=null)throw new InvalidOperationException("宿主基线探针被其他强制沙箱规则接管，不能作为有效基线。");
                File.WriteAllText(request.Output+".ack",request.Nonce);
                await live.WaitForExitAsync(timeout.Token);
                return result;
            }
            catch(OperationCanceledException) { throw new TimeoutException("验证超时或已取消，未将缺失结果记为通过。后台探针会自动退出。"); }
        }
    }
}
