<#
.SYNOPSIS
  Cadastra usuários no painel Suporte Appelsoft a partir de um CSV.

.USO
  .\importar-usuarios.ps1 -Csv .\usuarios.csv
  .\importar-usuarios.ps1 -Csv .\usuarios.csv -Simular     # só confere, não cria nada

  O script pede o usuário e a senha de um ADMIN do painel (não ficam salvos).

.CSV (separado por ponto e vírgula, primeira linha = cabeçalho)
  usuario;nome;email;grupo;admin;senha
  joao;João Silva;joao@appelsoft.com.br;Suporte;nao;
  maria;Maria Souza;maria@appelsoft.com.br;Suporte;nao;SenhaQueEuEscolhi1

  - usuario: obrigatório, 2 a 32 caracteres, sem espaço.
  - grupo:   nome do grupo de usuários que JÁ existe no painel (ex.: Suporte).
  - admin:   sim/nao (vazio = nao).
  - senha:   vazio = gera uma senha aleatória de 12 caracteres.

  Usuário que já existe é pulado (não altera nada).
  Ao final grava <csv>-resultado.csv com usuário e senha de quem foi criado.
  Esse arquivo tem senhas: entregue a cada um e APAGUE depois.
#>
param(
    [Parameter(Mandatory = $true)][string]$Csv,
    [string]$Servidor = "https://remoto.appelsoft.com.br",
    [switch]$Simular
)

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$api = "$Servidor/api/admin"

function Chamar([string]$Metodo, [string]$Caminho, $Corpo = $null) {
    $p = @{ Method = $Metodo; Uri = "$api$Caminho"; Headers = $script:headers; ContentType = "application/json; charset=utf-8" }
    if ($Corpo) { $p.Body = [Text.Encoding]::UTF8.GetBytes(($Corpo | ConvertTo-Json -Compress)) }
    $r = Invoke-RestMethod @p
    if ($r.code -ne 0) { throw "$($r.message)" }
    return $r.data
}

function NovaSenha {
    $chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789".ToCharArray()
    $bytes = New-Object byte[] 12
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    -join ($bytes | ForEach-Object { $chars[$_ % $chars.Length] })
}

# 1. Lê e confere o CSV
if (-not (Test-Path $Csv)) { throw "CSV não encontrado: $Csv" }
$linhas = @(Import-Csv -Path $Csv -Delimiter ";" -Encoding UTF8)
if (-not $linhas.Count) { throw "CSV vazio." }
foreach ($c in "usuario", "grupo") {
    if (-not ($linhas[0].PSObject.Properties.Name -contains $c)) { throw "Falta a coluna '$c' no CSV." }
}

# 2. Login do admin (senha não é gravada)
$script:headers = @{ "Accept-Language" = "pt-BR" }
$adminUser = Read-Host "Usuário admin do painel"
$adminSenha = Read-Host "Senha" -AsSecureString
$plain = [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($adminSenha))
$login = Chamar POST "/login" @{ username = $adminUser; password = $plain; platform = "script" }
$plain = $null
if (-not $login.token) { throw "Login sem token." }
$script:headers["api-token"] = $login.token

# 3. Grupos existentes
$grupos = @{}
foreach ($g in (Chamar GET "/group/list?page=1&page_size=1000").list) { $grupos[$g.name.ToLower()] = $g.id }

# 4. Cria os usuários
$resultado = @()
foreach ($l in $linhas) {
    $usuario = ([string]$l.usuario).Trim()
    if (-not $usuario) { continue }
    $grupoNome = ([string]$l.grupo).Trim()
    $status = ""
    try {
        if ($usuario -notmatch '^[A-Za-z0-9._@-]{2,32}$') { throw "usuário inválido (2-32, sem espaço)" }
        if (-not $grupos.ContainsKey($grupoNome.ToLower())) { throw "grupo '$grupoNome' não existe no painel" }
        $existe = (Chamar GET "/user/list?page=1&page_size=100&username=$([uri]::EscapeDataString($usuario))").list |
            Where-Object { $_.username -eq $usuario.ToLower() -or $_.username -eq $usuario }
        if ($existe) { Write-Host "= $usuario já existe, pulando" -ForegroundColor Yellow; continue }

        $senha = ([string]$l.senha).Trim()
        if (-not $senha) { $senha = NovaSenha }
        if ($senha.Length -lt 4 -or $senha.Length -gt 32) { throw "senha precisa ter de 4 a 32 caracteres" }
        $isAdmin = @("sim", "s", "1", "true", "yes") -contains ([string]$l.admin).Trim().ToLower()

        if ($Simular) { Write-Host "~ $usuario seria criado (grupo $grupoNome)"; continue }

        Chamar POST "/user/create" @{
            username = $usuario; nickname = ([string]$l.nome).Trim(); email = ([string]$l.email).Trim()
            group_id = $grupos[$grupoNome.ToLower()]; is_admin = $isAdmin; status = 1; remark = "importado"
        } | Out-Null
        $novo = (Chamar GET "/user/list?page=1&page_size=100&username=$([uri]::EscapeDataString($usuario))").list |
            Sort-Object id -Descending | Select-Object -First 1
        Chamar POST "/user/changePwd" @{ id = $novo.id; password = $senha } | Out-Null
        $resultado += [pscustomobject]@{ usuario = $novo.username; senha = $senha; grupo = $grupoNome }
        Write-Host "+ $($novo.username) criado" -ForegroundColor Green
    } catch {
        Write-Host "! $usuario : $($_.Exception.Message)" -ForegroundColor Red
    }
}

# 5. Resultado
if ($resultado.Count) {
    $saida = [IO.Path]::ChangeExtension((Resolve-Path $Csv).Path, $null).TrimEnd('.') + "-resultado.csv"
    $resultado | Export-Csv -Path $saida -Delimiter ";" -NoTypeInformation -Encoding UTF8
    Write-Host ""
    Write-Host "$($resultado.Count) usuário(s) criado(s). Senhas em: $saida" -ForegroundColor Green
    Write-Host "Entregue a cada um e APAGUE esse arquivo depois." -ForegroundColor Yellow
}
