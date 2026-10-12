using System.Runtime.InteropServices;
using System.Security.Principal;
using System.Security.Cryptography;
using System.Text;
using ClaudeFF.Core;
namespace ClaudeFF.Windows;
public static class LocalStore
{
    public static string Root => Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "ClaudeFF");
    public static string SettingsPath => Path.Combine(Root, "settings.json");
    public static string JournalPath => Path.Combine(Root, "policy-journal.json");
    public static string Runs => Path.Combine(Root, "Runs");
    public static string Canary => Path.Combine(Root, "Canary");
    public static string Box => "ClaudeFF_" + Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(WindowsIdentity.GetCurrent().User!.Value)))[..10];
    public static Settings Load() => File.Exists(SettingsPath) ? Json.Read<Settings>(SettingsPath) : new();
    public static void Save(Settings settings) => Json.Write(SettingsPath, settings);
    public static string HidePath(string text) => text.Replace(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "%USERPROFILE%", StringComparison.OrdinalIgnoreCase);
    public static string Protect(string text) => string.IsNullOrEmpty(text) ? "" : Transform(text, true);
    public static string Unprotect(string text) => string.IsNullOrEmpty(text) ? "" : Transform(text, false);
    [StructLayout(LayoutKind.Sequential)] private struct Blob { public int Length; public IntPtr Data; }
    [DllImport("crypt32.dll", SetLastError=true, CharSet=CharSet.Unicode)] private static extern bool CryptProtectData(ref Blob input, string? description, IntPtr entropy, IntPtr reserved, IntPtr prompt, int flags, out Blob output);
    [DllImport("crypt32.dll", SetLastError=true)] private static extern bool CryptUnprotectData(ref Blob input, IntPtr description, IntPtr entropy, IntPtr reserved, IntPtr prompt, int flags, out Blob output);
    [DllImport("kernel32.dll")] private static extern IntPtr LocalFree(IntPtr ptr);
    private static string Transform(string value, bool protect)
    {
        var bytes = protect ? Encoding.UTF8.GetBytes(value) : Convert.FromBase64String(value);
        var ptr = Marshal.AllocHGlobal(bytes.Length);
        try
        {
            Marshal.Copy(bytes,0,ptr,bytes.Length); var input=new Blob { Length=bytes.Length,Data=ptr };
            Blob result;
            bool ok=protect ? CryptProtectData(ref input,null,IntPtr.Zero,IntPtr.Zero,IntPtr.Zero,1,out result) : CryptUnprotectData(ref input,IntPtr.Zero,IntPtr.Zero,IntPtr.Zero,IntPtr.Zero,1,out result);
            if(!ok) throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error(),"无法访问当前 Windows 用户的加密凭证。");
            try { var output=new byte[result.Length];Marshal.Copy(result.Data,output,0,output.Length);try { return protect ? Convert.ToBase64String(output) : Encoding.UTF8.GetString(output); } finally { CryptographicOperations.ZeroMemory(output); } }
            finally { LocalFree(result.Data); }
        }
        finally { CryptographicOperations.ZeroMemory(bytes); Marshal.FreeHGlobal(ptr); }
    }
}
