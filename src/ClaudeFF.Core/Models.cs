using System.Text.Json;
using System.Text.Json.Serialization;

namespace ClaudeFF.Core;

public enum Verdict { Pass, Fail, Unknown, Manual, NotApplicable }
public record Check(string Id, string Name, Verdict Verdict, string Evidence, bool Required = false)
{
    [JsonIgnore] public string Status => Verdict switch { Verdict.Pass => "通过", Verdict.Fail => "失败", Verdict.Manual => "需手动验证", Verdict.NotApplicable => "不适用", _ => "未验证" };
}
public sealed class Report
{
    public string RunId { get; set; } = Guid.NewGuid().ToString("N");
    public DateTimeOffset CreatedAt { get; set; } = DateTimeOffset.UtcNow;
    public string Scope { get; set; } = "仅验证列出的路径和当前运行环境；不代表 Claude 功能验收或全面防泄漏。";
    public List<Check> Checks { get; set; } = [];
    [JsonIgnore] public bool RequiredPassed => Checks.Any(c => c.Required) && Checks.Where(c => c.Required).All(c => c.Verdict == Verdict.Pass);
    [JsonIgnore] public string Summary => Checks.Count==0 ? "尚无已启用防护可验证" : Checks.Any(c => c.Required && c.Verdict == Verdict.Fail) ? "检测发现失败项" : RequiredPassed ? "已启用防护的实测检查通过" : "验证未完成";
}
public enum NetworkMode { Existing, None, ManagedTcp }
public sealed class Settings
{
    public string SandboxHome { get; set; } = @"C:\Program Files\Sandboxie-Plus";
    public string ClaudePath { get; set; } = "";
    public bool ProtectBrowsers { get; set; } = true;
    public List<string> PrivateFolders { get; set; } = [];
    public NetworkMode NetworkMode { get; set; } = NetworkMode.Existing;
    public string ProxyHost { get; set; } = "";
    public int ProxyPort { get; set; } = 1080;
    public string ProxyUser { get; set; } = "";
    public string ProtectedPassword { get; set; } = "";
    public bool LightTheme { get; set; }
}
public record Target(string Label, string Path);
public record AccessSample(string Label, string Path, string Outcome, int ErrorCode = 0);
public sealed class ProbeRequest
{
    public string Nonce { get; set; } = "";
    public string Box { get; set; } = "";
    public string Output { get; set; } = "";
    public List<Target> Files { get; set; } = [];
    public List<Target> Directories { get; set; } = [];
    public string RegistryKey { get; set; } = "";
    public List<Target> RegistryTargets { get; set; } = [];
    public bool Network { get; set; }
    public bool RelayChallenge { get; set; }
}
public sealed class ProbeResult
{
    public string Nonce { get; set; } = "";
    public int Pid { get; set; }
    public string? Box { get; set; }
    public List<AccessSample> Access { get; set; } = [];
    public string RegistryOutcome { get; set; } = "unknown";
    public List<AccessSample> RegistryAccess { get; set; } = [];
    public string Locale { get; set; } = "";
    public string? RelayResponse { get; set; }
    public string? ExitIp { get; set; }
    public string? NetworkError { get; set; }
}
public static class Json
{
    public static readonly JsonSerializerOptions Options = new() { WriteIndented = true, Converters = { new JsonStringEnumConverter() } };
    public static T Read<T>(string path) => JsonSerializer.Deserialize<T>(File.ReadAllText(path), Options) ?? throw new InvalidDataException("配置为空。");
    public static void Write<T>(string path, T value)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        var temp = path + "." + Guid.NewGuid().ToString("N") + ".tmp";
        try { File.WriteAllText(temp, JsonSerializer.Serialize(value, Options)); File.Move(temp, path, true); }
        finally { if (File.Exists(temp)) File.Delete(temp); }
    }
}
public static class VerificationRules
{
    public static Check FileDenied(string id, AccessSample host, AccessSample? sandbox)
    {
        if (host.Outcome != "readable") return new(id, host.Label, Verdict.Unknown, "宿主基线不可读，不能判断是否由沙箱阻止。", true);
        if (sandbox is null) return new(id, host.Label, Verdict.Unknown, "缺少沙箱实测结果。", true);
        // Missing paths and unrelated IO failures are never counted as access denial.
        return sandbox.Outcome switch
        {
            "denied" => new(id, host.Label, Verdict.Pass, "宿主可读，沙箱访问被拒绝。", true),
            "readable" => new(id, host.Label, Verdict.Fail, "沙箱内仍可访问此资源。", true),
            _ => new(id, host.Label, Verdict.Unknown, "沙箱返回 " + sandbox.Outcome + "，不能证明权限阻止。", true)
        };
    }
    public static bool CanRestore(IReadOnlyList<string> actual,IReadOnlyList<string> before,IReadOnlyList<string> after,bool completed) =>
        actual.SequenceEqual(before) || actual.SequenceEqual(after) ||
        (!completed && actual.Count<=after.Count && actual.SequenceEqual(after.Take(actual.Count)));
    public static void ValidateText(string value)
    {
        if (value.IndexOfAny(['\r','\n','\0']) >= 0) throw new ArgumentException("配置不允许换行或空字符。");
    }
    public static string FolderRule(string path)
    {
        ValidateText(path);
        if (!System.Text.RegularExpressions.Regex.IsMatch(path, @"^[A-Za-z]:\\") || path.Contains('*') || path.Contains('?') || path.Contains(';') || path.Contains(',') || path.Contains('%'))
            throw new ArgumentException("请选择本地磁盘上的完整目录路径，不能包含规则分隔符或通配符。");
        var trimmed = path.TrimEnd('\\','/');
        if (trimmed.Length <= 3 || path.Split('\\').Any(p => p is ".." or ".")) throw new ArgumentException("不能限制整个磁盘或使用相对路径。");
        return trimmed + "\\";
    }
}
