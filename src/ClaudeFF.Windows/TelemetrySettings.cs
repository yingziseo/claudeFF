using System.Diagnostics;
using System.Windows.Automation;
using ClaudeFF.Core;
namespace ClaudeFF.Windows;
public record TelemetryChange(string Name, bool WasOn, DateTimeOffset ChangedAt, bool Verified=false);
public static class TelemetrySettings
{
    private static string Journal=>Path.Combine(LocalStore.Root,"telemetry.json");
    // Officially documented model-improvement preference. This is NOT an all-telemetry switch.
    private static readonly string[] KnownLabels=["Help Improve our AI models","Help improve our AI models","Help improve Claude","Improve Claude for everyone"];
    private static List<TelemetryChange> Changes()=>File.Exists(Journal)?Json.Read<List<TelemetryChange>>(Journal):[];
    private static (AutomationElement Element,TogglePattern Toggle)? Find(string? expected=null)
    {
        foreach(var process in Process.GetProcessesByName("Claude")) using(process)
        {
            if(process.MainWindowHandle==IntPtr.Zero)continue;
            var window=AutomationElement.FromHandle(process.MainWindowHandle);
            foreach(var label in expected is null ? KnownLabels : [expected])
            {
                var elements=window.FindAll(TreeScope.Descendants,new PropertyCondition(AutomationElement.NameProperty,label));
                foreach(AutomationElement element in elements)
                    if(element.Current.IsEnabled && !element.Current.IsOffscreen && element.TryGetCurrentPattern(TogglePattern.Pattern,out var pattern)) return (element,(TogglePattern)pattern);
            }
        }
        return null;
    }
    public static string DisableOptional()
    {
        using var operationLock=PolicyManager.Lock();
        var found=Find();
        if(found is null)return "未找到可验证的模型改进开关。请在 Claude 中打开 Settings → Privacy，再重试；当前没有修改任何设置。此按钮不代表关闭全部遥测。";
        var (element,toggle)=found.Value;
        if(toggle.Current.ToggleState==ToggleState.Indeterminate)return "开关状态不确定，未修改。";
        if(toggle.Current.ToggleState==ToggleState.Off)return "模型改进开关已经关闭，未新增修改；未将其登记为本工具启用的防护。";
        var name=element.Current.Name;
        var changes=Changes();
        if(changes.Any(c=>c.Name==name))throw new InvalidOperationException("该设置已有变更记录，请先验证或恢复。");
        // Keep a recovery record before clicking; do not repeat the toggle on an ambiguous result.
        changes.Add(new(name,true,DateTimeOffset.UtcNow));Json.Write(Journal,changes);
        toggle.Toggle();
        if(toggle.Current.ToggleState!=ToggleState.Off)return "已发出关闭操作，但读回未确认。请检查设置；记录已保留，可重试验证或恢复。";
        changes[^1]=changes[^1] with {Verified=true};Json.Write(Journal,changes);
        return "已关闭并读回确认：模型改进偏好。账户级设置可能同步到其他设备；不代表全部遥测已关闭。";
    }
    public static List<Check> Verify()
    {
        var results=new List<Check>();
        foreach(var change in Changes().Where(c=>c.Verified))
        {
            var found=Find(change.Name);
            var verdict=found is null?Verdict.Unknown:found.Value.Toggle.Current.ToggleState==ToggleState.Off?Verdict.Pass:Verdict.Fail;
            results.Add(new("telemetry-model","已应用的模型改进偏好",verdict,found is null?"请在 Claude 中打开 Privacy 页面再验证；当前无法读回开关。":"通过 Windows UI Automation 读回实际开关；验证范围仅为此账户偏好。",true));
        }
        return results;
    }
    public static string Restore()
    {
        using var operationLock=PolicyManager.Lock();
        var changes=Changes();
        foreach(var change in changes.ToList())
        {
            var found=Find(change.Name);
            if(found is null)throw new InvalidOperationException("请先打开 Claude 的 Privacy 页面，以便恢复本工具修改的开关。");
            if(found.Value.Toggle.Current.ToggleState==ToggleState.Indeterminate)throw new InvalidOperationException("开关状态不明确，停止恢复。");
            if(change.WasOn && found.Value.Toggle.Current.ToggleState==ToggleState.Off)found.Value.Toggle.Toggle();
            if(found.Value.Toggle.Current.ToggleState!=ToggleState.On)throw new InvalidOperationException("恢复未通过读回验证。");
            changes.Remove(change);Json.Write(Journal,changes);
        }
        return "本工具修改的模型改进偏好已恢复。";
    }
}
