using System.Net;
using System.Net.Sockets;
using ClaudeFF.Core;
using ClaudeFF.Windows;
if(args.Length!=2||args[0]!="--session")return;
var file=Path.GetFullPath(args[1]);
if(!file.StartsWith(LocalStore.Root+Path.DirectorySeparatorChar,StringComparison.OrdinalIgnoreCase))return;
var config=Json.Read<RelaySession>(file);
using var quit=new CancellationTokenSource();
var listener=new TcpListener(IPAddress.Loopback,0);
listener.Start();config.Port=((IPEndPoint)listener.LocalEndpoint).Port;config.Pid=Environment.ProcessId;
var bridge=new SocksBridge(config.ProxyHost,config.ProxyPort,config.ProxyUser,LocalStore.Unprotect(config.ProtectedPassword));
Json.Write(file+".ready",config);
var monitor=Task.Run(async()=>{while(!File.Exists(file+".stop")){await Task.Delay(500);}quit.Cancel();listener.Stop();});
using var slots=new SemaphoreSlim(64);
try
{
    while(!quit.IsCancellationRequested)
    {
        await slots.WaitAsync(quit.Token);
        TcpClient client;
        try{client=await listener.AcceptTcpClientAsync(quit.Token);}catch{slots.Release();throw;}
        _=Task.Run(async()=>{try{await bridge.Handle(client,quit.Token);}catch{client.Dispose();}finally{slots.Release();}});
    }
}
catch(OperationCanceledException){}catch(SocketException){}catch(ObjectDisposedException){}
finally{listener.Stop();}
