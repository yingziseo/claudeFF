using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
namespace ClaudeFF.Windows;
public sealed class SandboxApi : IDisposable
{
    private readonly IntPtr library;
    public string Home { get; }
    [UnmanagedFunctionPointer(CallingConvention.StdCall, CharSet=CharSet.Unicode)] private delegate int QueryConf(string section,string? key,uint index,StringBuilder value,uint length);
    [UnmanagedFunctionPointer(CallingConvention.StdCall, CharSet=CharSet.Unicode)] private delegate int UpdateConf(ushort op,string? password,string section,string key,string? value);
    [UnmanagedFunctionPointer(CallingConvention.StdCall, CharSet=CharSet.Unicode)] private delegate int QueryProcess(IntPtr pid,StringBuilder box,IntPtr image,IntPtr sid,IntPtr session);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] private delegate int Reload(uint session);
    public SandboxApi(string home)
    {
        Home=Path.GetFullPath(home);
        if (!File.Exists(Path.Combine(Home,"Start.exe")) || !File.Exists(Path.Combine(Home,"SbieDll.dll"))) throw new InvalidOperationException("未找到 Sandboxie。请先安装官方 x64 版本，并选择其安装目录。");
        library=NativeLibrary.Load(Path.Combine(Home,"SbieDll.dll"));
    }
    private T Function<T>(string name) where T:Delegate => Marshal.GetDelegateForFunctionPointer<T>(NativeLibrary.GetExport(library,name));
    public List<string> Read(string box,string? key)
    {
        var result=new List<string>();var query=Function<QueryConf>("SbieApi_QueryConf");
        for(uint i=0;i<4096;i++)
        {
            var buffer=new StringBuilder(8192);int status=query(box,key,i|0x70000000,buffer,16384);
            if(unchecked((uint)status)==0xC000008B) return result;
            if(status!=0) throw new InvalidOperationException($"读取沙箱配置失败（0x{status:X8}）。请检查 Sandboxie 驱动和服务。");
            result.Add(buffer.ToString());
        }
        throw new InvalidOperationException("沙箱配置条目过多。");
    }
    public void Write(string box,string key,IReadOnlyList<string> values)
    {
        var update=Function<UpdateConf>("SbieDll_UpdateConf");
        int code=update('s',null,box,key,values.Count==0 ? null : values[0]);
        if(code!=0) throw new InvalidOperationException($"写入 {key} 失败（0x{code:X8}）。请检查配置锁或以同一用户管理员身份重试。");
        foreach(var value in values.Skip(1))
        {
            code=update('a',null,box,key,value);
            if(code!=0) throw new InvalidOperationException($"追加 {key} 失败（0x{code:X8}）。请恢复本次更改。");
        }
    }
    public void ReloadConfiguration()
    {
        var status=Function<Reload>("SbieApi_ReloadConf")(uint.MaxValue);
        if(status!=0) throw new InvalidOperationException($"重载沙箱配置失败（0x{status:X8}）。");
    }
    public string? ProcessBox(int pid)
    {
        var box=new StringBuilder(34);
        var status=Function<QueryProcess>("SbieApi_QueryProcess")(new IntPtr(pid),box,IntPtr.Zero,IntPtr.Zero,IntPtr.Zero);
        return status==0 ? box.ToString() : null;
    }
    public List<int> BoxProcesses(string box)
    {
        var result=new List<int>();
        foreach(var process in Process.GetProcesses()) using(process) { try { if(ProcessBox(process.Id)==box)result.Add(process.Id); }catch(ArgumentException){} }
        return result;
    }
    public Process Start(string box,string executable,params string[] arguments)
    {
        var info=new ProcessStartInfo(Path.Combine(Home,"Start.exe")) { UseShellExecute=false,CreateNoWindow=true };
        info.ArgumentList.Add("/box:"+box);info.ArgumentList.Add("/silent");info.ArgumentList.Add(executable);
        foreach(var arg in arguments) info.ArgumentList.Add(arg);
        return Process.Start(info) ?? throw new InvalidOperationException("Sandboxie 未能启动程序。");
    }
    public void Dispose()=>NativeLibrary.Free(library);
}
