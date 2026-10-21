<#macro registrationLayout bodyClass="" displayInfo=false displayMessage=true displayRequiredFields=false>
<!DOCTYPE html>
<html class="${properties.kcHtmlClass!}">
<head>
    <meta charset="utf-8" />
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <meta name="robots" content="noindex, nofollow" />
    <meta name="viewport" content="width=device-width,initial-scale=1" />

    <title>${msg("loginTitle",(realm.displayName!''))}</title>

    <#if properties.meta?has_content>
        <#list properties.meta?split(' ') as meta>
            <meta name="${meta?split('==')[0]}" content="${meta?split('==')[1]}" />
        </#list>
    </#if>

    <link rel="icon" href="${url.resourcesPath}/img/logo.png" />
    <#if properties.stylesCommon?has_content>
        <#list properties.stylesCommon?split(' ') as style>
            <link rel="stylesheet" href="${url.resourcesCommonPath}/${style}" />
        </#list>
    </#if>
    <#if properties.styles?has_content>
        <#list properties.styles?split(' ') as style>
            <link rel="stylesheet" href="${url.resourcesPath}/${style}" />
        </#list>
    </#if>
    <script src="${url.resourcesPath}/js/login.js" defer></script>
</head>

<body class="moh-login-body ${bodyClass}">
    <div class="moh-shell">
        <aside class="moh-brand-panel">
            <div class="moh-brand-top">
                <img src="${url.resourcesPath}/img/logo.png" alt="MOH Logo" class="moh-logo" />
                <div class="moh-brand-text">
                    <div class="moh-brand-kicker">Republic of Uganda</div>
                    <h1>Ministry of Health</h1>
                    <p>Integrated Health Portal</p>
                </div>
            </div>

            <div class="moh-brand-content">
                <h2>Secure access to connected health systems</h2>
                <p>
                    Sign in once to access approved Ministry of Health applications,
                    dashboards, and services.
                </p>

                <ul class="moh-feature-list">
                    <li>Centralized authentication</li>
                    <li>Role-based application access</li>
                    <li>Session visibility and security controls</li>
                </ul>
            </div>

            <div class="moh-brand-footer">
                <span>Protected system access</span>
            </div>
        </aside>

        <main class="moh-form-panel">
            <div class="moh-form-card">
                <div class="moh-form-header">
                    <div class="moh-form-eyebrow">Account access</div>
                    <h2>${realm.displayName!"MOH SSO"}</h2>
                    <p>Use your authorized account credentials to continue.</p>
                </div>

                <#if displayMessage && message?has_content>
                    <div class="moh-alert moh-alert-${message.type}">
                        ${kcSanitize(message.summary)?no_esc}
                    </div>
                </#if>

                <#nested "form">

                <#if displayInfo>
                    <div class="moh-info-block">
                        <#nested "info">
                    </div>
                </#if>
            </div>

            <div class="moh-meta">
                <p>Unauthorized access is prohibited. Activity may be monitored and audited.</p>
            </div>
        </main>
    </div>

    <#include "footer.ftl">
</body>
</html>
</#macro>
