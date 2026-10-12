using ClaudeFF.Core;
using Microsoft.Win32;
namespace ClaudeFF.Windows;
public sealed class Change
{
    public string Key { get; set; }="";
    public List<string> Before { get; set; }=[];
    public List<string> After { get; set; }=[];
    public bool Completed { get; set; }
}
public sealed class PolicyJournal
{
    public string Box { get; set; }="";
    public string SandboxHome { get; set; }="";
    public string State { get; set; }="Applying";
    public string? RelayId { get; set; }
    public List<Target> Targets { get; set; }=[];
    public List<Target> RegistryTargets { get; set; }=[];
    public List<Change> Changes { get; set; }=[];
    public DateTimeOffset AppliedAt { get; set; }=DateTimeOffset.UtcNow;
}
public static class PolicyManager
{
    public const string RegistryCanary = @"Software\ClaudeFF\VerificationCanary";
    public static List<Target> BrowserTargets()
    {
        var local=Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        var roaming=Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        return new[] {
            new Target("Chrome 浏览器资料",Path.Combine(local,@"Google\Chrome\User Data")),
            new Target("Edge 浏览器资料",Path.Combine(local,@"Microsoft\Edge\User Data")),
            new Target("Brave 浏览器资料",Path.Combine(local,@"BraveSoftware\Brave-Browser\User Data")),
            new Target("Firefox 浏览器资料",Path.Combine(roaming,@"Mozilla\Firefox\Profiles")),
            new Target("Firefox 本地缓存",Path.Combine(local,@"Mozilla\Firefox\Profiles"))
        }.Where(t=>Directory.Exists(t.Path)).ToList();
    }
    public static List<Target> BrowserRegistryTargets()
    {
        var result=new List<Target>();
        foreach(var keyPath in new[]{@"Software\Google\Chrome\PreferenceMACs",@"Software\Microsoft\Edge\PreferenceMACs"})
        {
            try{using var key=Registry.CurrentUser.OpenSubKey(keyPath);if(key!=null)result.Add(new("浏览器偏好注册表",keyPath));}
            catch(UnauthorizedAccessException){}catch(System.Security.SecurityException){}
        }
        return result;
    }
    public static PolicyJournal? Current() => File.Exists(LocalStore.JournalPath) ? Json.Read<PolicyJournal>(LocalStore.JournalPath) : null;
    public static void Apply(Settings settings)
    {
        using var policyLock=Lock();
        if(Current()!=null) throw new InvalidOperationException("已有生效或未完成的配置。请先恢复，再重新应用。");
        using var api=new SandboxApi(settings.SandboxHome);
        var box=LocalStore.Box;
        if(api.Read(box,null).Count>0) throw new InvalidOperationException("同名沙箱已存在且不由当前记录管理，已停止以免覆盖。");
        if(api.BoxProcesses(box).Count>0) throw new InvalidOperationException("请先关闭此沙箱中的程序。");
        var targets=settings.ProtectBrowsers ? BrowserTargets() : [];
        foreach(var folder in settings.PrivateFolders)
        {
            var full=Path.GetFullPath(folder);
            VerificationRules.FolderRule(full);
            foreach(var vital in new[]{Environment.GetFolderPath(Environment.SpecialFolder.Windows),Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles),LocalStore.Root,AppContext.BaseDirectory,Path.GetDirectoryName(settings.ClaudePath)??""})
                if(vital.Length>0 && (IsUnder(vital,full) || IsUnder(full,vital))) throw new ArgumentException("不能限制 Windows、程序目录、工具数据或其父目录。请选择具体的私人资料目录。");
            if(!Directory.Exists(full))throw new DirectoryNotFoundException("指定的私人目录不存在。");
            targets.Add(new("私人目录 "+(targets.Count+1),full));
        }
        targets=targets.DistinctBy(t=>t.Path,StringComparer.OrdinalIgnoreCase).ToList();
        Directory.CreateDirectory(LocalStore.Canary);Directory.CreateDirectory(LocalStore.Runs);
        var registryTargets=settings.ProtectBrowsers?BrowserRegistryTargets():[];
        var planned=new Dictionary<string,List<string>> {
            ["Enabled"]=["y"], ["BoxNameTitle"]=["claudeFF · 隐私隔离"],
            ["ConfigLevel"]=["10"],
            ["ClosedFilePath"]=targets.Select(t=>VerificationRules.FolderRule(t.Path)).Append(VerificationRules.FolderRule(LocalStore.Canary)).ToList(),
            ["ClosedKeyPath"]=registryTargets.Select(t=>@"HKEY_CURRENT_USER\"+t.Path).Append(@"HKEY_CURRENT_USER\"+RegistryCanary).ToList(),
            // Only the shipped probe needs this narrow host output channel. No user documents are opened.
            ["OpenFilePath"]=[ExecutableNames.Probe+","+VerificationRules.FolderRule(LocalStore.Runs)]
        };
        var journal=new PolicyJournal { Box=box,SandboxHome=settings.SandboxHome,Targets=targets,RegistryTargets=registryTargets };
        foreach(var pair in planned) journal.Changes.Add(new(){Key=pair.Key,Before=api.Read(box,pair.Key),After=pair.Value});
        Json.Write(LocalStore.JournalPath,journal); // write-ahead record survives a crash
        try
        {
            if(settings.NetworkMode==NetworkMode.ManagedTcp)
            {
                var oldProxy=api.Read(box,"NetworkUseProxy");
                var relay=RelayManager.Start(settings,id=>{journal.RelayId=id;Json.Write(LocalStore.JournalPath,journal);});
                journal.Changes.Add(new(){Key="NetworkUseProxy",Before=oldProxy,After=[$"*,Address=127.0.0.1;Port={relay.Port};Auth=No"]});
                Json.Write(LocalStore.JournalPath,journal);
            }
            foreach(var change in journal.Changes) { api.Write(box,change.Key,change.After);change.Completed=true;Json.Write(LocalStore.JournalPath,journal); }
            api.ReloadConfiguration();journal.State="Applied";Json.Write(LocalStore.JournalPath,journal);
        }
        catch(Exception error)
        {
            journal.State="Incomplete";Json.Write(LocalStore.JournalPath,journal);
            try { RestoreCore(journal,api); }
            catch(Exception recovery) { throw new InvalidOperationException("应用失败，自动恢复未完成："+recovery.Message+" 原因："+error.Message,error); }
            throw new InvalidOperationException("应用失败，已恢复本次修改："+error.Message,error);
        }
    }
    public static void Restore()
    {
        using var policyLock=Lock();
        var journal=Current()??throw new InvalidOperationException("没有需要恢复的配置。");
        if(journal.Box!=LocalStore.Box)throw new InvalidOperationException("配置所属用户不匹配。");
        using var api=new SandboxApi(journal.SandboxHome);
        if(api.BoxProcesses(journal.Box).Count>0)throw new InvalidOperationException("请先退出受保护 Claude 和该沙箱中的其他程序，再恢复。不会强制终止你的工作。");
        RestoreCore(journal,api);
    }
    private static void RestoreCore(PolicyJournal journal,SandboxApi api)
    {
        if(api.BoxProcesses(journal.Box).Count>0)throw new InvalidOperationException("专属沙箱仍有进程运行，请退出后恢复。");
        // Preflight the entire transaction before changing anything; preserve external edits.
        foreach(var change in journal.Changes)
        {
            var actual=api.Read(journal.Box,change.Key);
            if(!VerificationRules.CanRestore(actual,change.Before,change.After,change.Completed))
                throw new InvalidOperationException($"{change.Key} 已被外部修改，停止自动恢复以保留你的更改。请在 Sandboxie 中检查专属沙箱。");
        }
        journal.State="Restoring";Json.Write(LocalStore.JournalPath,journal);
        foreach(var change in journal.Changes.AsEnumerable().Reverse()) api.Write(journal.Box,change.Key,change.Before);
        api.ReloadConfiguration();if(journal.RelayId is {} relayId)RelayManager.Stop(relayId);File.Delete(LocalStore.JournalPath);
        // Deliberately keep sandbox contents: login caches and user downloads must not be deleted.
    }
    public static bool IsUnder(string path,string parent)=>string.Equals(path.TrimEnd('\\'),parent.TrimEnd('\\'),StringComparison.OrdinalIgnoreCase) || path.StartsWith(parent.TrimEnd('\\')+"\\",StringComparison.OrdinalIgnoreCase);
    public static FileStream Lock()
    {
        Directory.CreateDirectory(LocalStore.Root);
        try{return new FileStream(Path.Combine(LocalStore.Root,"operation.lock"),FileMode.OpenOrCreate,FileAccess.ReadWrite,FileShare.None);}
        catch(IOException){throw new InvalidOperationException("另一个配置或验证操作正在进行，请等待完成。");}
    }
}
