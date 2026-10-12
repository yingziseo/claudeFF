using System.Net;
using System.Net.Sockets;
using System.Text;
namespace ClaudeFF.Core;

// A narrow TCP-only bridge keeps upstream credentials out of Sandboxie.ini.
// It is NOT a VPN, UDP proxy or firewall. No direct fallback is implemented.
public sealed class SocksBridge(string host,int port,string username,string password)
{
    public const string CanaryAddress="192.0.2.123"; // Documentation-only address; handled locally, never forwarded.
    public const int CanaryPort=43439;
    public async Task Handle(TcpClient downstream,CancellationToken cancellation)
    {
        using(downstream)
        using(var handshake=CancellationTokenSource.CreateLinkedTokenSource(cancellation))
        {
            handshake.CancelAfter(TimeSpan.FromSeconds(20));var ct=handshake.Token;
            var input=downstream.GetStream();bool established=false;
            try
            {
                var greeting=await Read(input,2,ct);
                if(greeting[0]!=5||greeting[1]==0)return;
                var methods=await Read(input,greeting[1],ct);
                if(!methods.Contains((byte)0)){await input.WriteAsync(new byte[]{5,255},ct);return;}
                await input.WriteAsync(new byte[]{5,0},ct);
                var header=await Read(input,4,ct);
                if(header[0]!=5||header[1]!=1||header[2]!=0){await input.WriteAsync(new byte[]{5,7,0,1,0,0,0,0,0,0},ct);return;}
                var address=await ReadAddress(input,header[3],ct);var endpointPort=await Read(input,2,ct);
                if(header[3]==1 && new IPAddress(address).ToString()==CanaryAddress && (endpointPort[0]*256+endpointPort[1])==CanaryPort)
                {
                    await input.WriteAsync(new byte[]{5,0,0,1,127,0,0,1,0,0},ct);
                    var challenge=await Read(input,32,ct);
                    await input.WriteAsync(Encoding.ASCII.GetBytes("claudeFF-relay:"),ct);await input.WriteAsync(challenge,ct);return;
                }
                using var upstream=new TcpClient();await upstream.ConnectAsync(host,port,ct);var output=upstream.GetStream();
                bool auth=!string.IsNullOrEmpty(username);
                await output.WriteAsync(new byte[]{5,1,auth?(byte)2:(byte)0},ct);
                var selected=await Read(output,2,ct);
                if(selected[0]!=5||selected[1]!=(auth?2:0))throw new IOException("SOCKS authentication method rejected.");
                if(auth)
                {
                    var user=Encoding.UTF8.GetBytes(username);var pass=Encoding.UTF8.GetBytes(password);
                    if(user.Length is <1 or >255 || pass.Length>255)throw new IOException("Invalid credential length.");
                    await output.WriteAsync(new byte[]{1,(byte)user.Length}.Concat(user).Concat(new byte[]{(byte)pass.Length}).Concat(pass).ToArray(),ct);
                    var authenticated=await Read(output,2,ct);if(authenticated[0]!=1||authenticated[1]!=0)throw new IOException("SOCKS authentication rejected.");
                }
                await output.WriteAsync(header.Concat(address).Concat(endpointPort).ToArray(),ct);
                var reply=await Read(output,4,ct);if(reply[0]!=5||reply[2]!=0)throw new IOException("Invalid SOCKS reply.");
                var replyAddress=await ReadAddress(output,reply[3],ct);var replyPort=await Read(output,2,ct);
                await input.WriteAsync(reply.Concat(replyAddress).Concat(replyPort).ToArray(),ct);
                if(reply[1]!=0)return;
                established=true;
                // Handshake deadlines do not truncate long model replies or attachment transfers.
                using var relayStop=CancellationTokenSource.CreateLinkedTokenSource(cancellation);
                await Task.WhenAll(Pump(input,output,upstream,relayStop),Pump(output,input,downstream,relayStop));
            }
            catch(Exception ex) when(ex is IOException or SocketException or OperationCanceledException)
            {
                try{if(established)return;using var fail=new CancellationTokenSource(TimeSpan.FromSeconds(1));await input.WriteAsync(new byte[]{5,1,0,1,0,0,0,0,0,0},fail.Token);}catch{}
            }
        }
    }
    private static async Task Pump(NetworkStream source,NetworkStream target,TcpClient receiver,CancellationTokenSource stop)
    {
        try{await source.CopyToAsync(target,stop.Token);receiver.Client.Shutdown(SocketShutdown.Send);}catch{stop.Cancel();throw;}
    }
    public static async Task<byte[]> Read(Stream stream,int length,CancellationToken ct)
    {
        var data=new byte[length];await stream.ReadExactlyAsync(data,ct);return data;
    }
    private static async Task<byte[]> ReadAddress(Stream stream,byte type,CancellationToken ct)
    {
        if(type==1)return await Read(stream,4,ct);
        if(type==4)return await Read(stream,16,ct);
        if(type!=3)throw new IOException("Invalid SOCKS address type.");
        var size=await Read(stream,1,ct);if(size[0]==0)throw new IOException("Empty SOCKS domain.");
        return size.Concat(await Read(stream,size[0],ct)).ToArray();
    }
}
public sealed class RelaySession
{
    public string Id {get;set;}=Guid.NewGuid().ToString("N");
    public int Pid {get;set;}
    public int Port {get;set;}
    public string ProxyHost {get;set;}="";
    public int ProxyPort {get;set;}
    public string ProxyUser {get;set;}="";
    public string ProtectedPassword {get;set;}="";
}
