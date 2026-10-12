using System.Net;
using System.Text.Json;
namespace ClaudeFF.Core;
public static class NetworkProbe
{
    // No direct fallback. A failed request is an unknown result, never a passed privacy check.
    public static async Task<string> ExitIpAsync(IWebProxy? proxy, bool useSystemProxy, CancellationToken token)
    {
        using var handler = new HttpClientHandler { UseProxy = proxy is not null || useSystemProxy, Proxy = proxy, AllowAutoRedirect = false };
        using var client = new HttpClient(handler) { Timeout = TimeSpan.FromSeconds(15) };
        using var response = await client.GetAsync("https://api.ipify.org?format=json", token);
        response.EnsureSuccessStatusCode();
        using var json = JsonDocument.Parse(await response.Content.ReadAsStringAsync(token));
        var ip = json.RootElement.GetProperty("ip").GetString();
        if (!IPAddress.TryParse(ip, out _)) throw new InvalidDataException("出口接口未返回有效 IP。");
        return ip!;
    }
    public static WebProxy Socks(string host, int port, string user, string password)
    {
        if (string.IsNullOrWhiteSpace(host) || host.IndexOfAny(['/', '\\', '@', ';', '\r', '\n', ' ']) >= 0 || port is < 1 or > 65535)
            throw new ArgumentException("代理地址或端口无效。");
        var proxy = new WebProxy(new UriBuilder("socks5", host, port).Uri);
        if (!string.IsNullOrEmpty(user)) proxy.Credentials = new NetworkCredential(user, password);
        return proxy;
    }
}
