module Main exposing (Namespace, Status, main, msgDecoder, nsDecoder, statusDecoder)

import Browser
import Element
    exposing
        ( Attr
        , Attribute
        , Color
        , Element
        , alignRight
        , column
        , el
        , fill
        , fillPortion
        , height
        , layout
        , maximum
        , none
        , padding
        , rgb255
        , row
        , shrink
        , spacing
        , table
        , text
        , width
        )
import Element.Background as Background
import Element.Border as Border
import Element.Events as Events
import Element.Font as Font
import Element.Input as Input
import Html
import Http exposing (Error(..))
import Json.Decode as D
import Json.Encode as E
import Time



-- MAIN


main : Program Flags Model Msg
main =
    Browser.element
        { init = init
        , update = update
        , subscriptions = subscriptions
        , view = view
        }



-- MODEL


type ServerState
    = Loading
    | LoadFailed String
    | Loaded Status


type alias Model =
    { state : ServerState
    , editLimit : Maybe String
    , editStart : Maybe String
    , url : String
    }


type alias Flags =
    String


type alias Namespace =
    { name : String
    , hasDownQuota : Bool
    , canExtend : Bool
    , memUsed : Int
    , memLimit : Int
    , autoStartHour : Maybe Int
    , remaining : Maybe String
    }


type alias Status =
    { clock : String
    , namespaces : List Namespace
    }


loadState : String -> Cmd Msg
loadState url =
    Http.get
        { url = url ++ "status"
        , expect = Http.expectString GotUpdate
        }


encode : String -> E.Value
encode namespace =
    E.object [ ( "namespace", E.string namespace ) ]


extend : String -> String -> Cmd Msg
extend url namespace =
    Http.post
        { url = url ++ "extend"
        , body = encode namespace |> Http.jsonBody
        , expect = Http.expectString GotUpdate
        }


encodeLimit : String -> Int -> E.Value
encodeLimit namespace limit =
    E.object [ ( "namespace", E.string namespace ), ( "limit", E.int limit ) ]


setLimit : String -> String -> Int -> Cmd Msg
setLimit url namespace value =
    Http.post
        { url = url ++ "setMemLimit"
        , body = encodeLimit namespace value |> Http.jsonBody
        , expect = Http.expectString GotUpdate
        }


encodeStart : String -> Maybe Int -> E.Value
encodeStart namespace value =
    let
        start =
            case value of
                Just x ->
                    E.int x

                Nothing ->
                    E.null
    in
    E.object [ ( "namespace", E.string namespace ), ( "startHour", start ) ]


restart : String -> Cmd Msg
restart url =
    Http.post
        { url = url ++ "restart"
        , body = E.object []|> Http.jsonBody
        , expect = Http.expectString GotUpdate
        }

setStart : String -> String -> Maybe Int -> Cmd Msg
setStart url namespace value =
    Http.post
        { url = url ++ "setStartHour"
        , body = encodeStart namespace value |> Http.jsonBody
        , expect = Http.expectString GotUpdate
        }


init : Flags -> ( Model, Cmd Msg )
init flags =
    ( { state = Loading
      , editLimit = Nothing
      , editStart = Nothing
      , url = flags
      }
    , loadState flags
    )



-- UPDATE


type Msg
    = GotUpdate (Result Http.Error String)
    | GetUpdate Time.Posix
    | Extend String
    | EditLimit (Maybe String)
    | SetLimit String Int
    | EditStart (Maybe String)
    | SetStart String (Maybe Int)
    | Restart 


toString : Http.Error -> String
toString e =
    case e of
        BadUrl msg ->
            "Bad url: " ++ msg

        Timeout ->
            "Timeout"

        NetworkError ->
            "Network error"

        BadStatus x ->
            "Bad status: " ++ String.fromInt x

        BadBody msg ->
            "Bad body: " ++ msg


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotUpdate result ->
            case result of
                Ok json ->
                    case msgDecoder json of
                        Ok status ->
                            ( { model | state = Loaded status }, Cmd.none )

                        Err x ->
                            ( { model | state = LoadFailed <| D.errorToString x }, Cmd.none )

                Err x ->
                    ( { model | state = LoadFailed <| toString x }, Cmd.none )

        GetUpdate _ ->
            ( model, loadState model.url )

        Extend namespace ->
            ( model, extend model.url namespace )

        EditLimit namespace ->
            ( { model | editLimit = namespace }, Cmd.none )

        SetLimit namespace limit ->
            ( model, setLimit model.url namespace limit )

        EditStart namespace ->
            ( { model | editStart = namespace }, Cmd.none )

        SetStart namespace start ->
            ( model, setStart model.url namespace start )

        Restart ->
            ( model, restart model.url)



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions _ =
    Time.every 5000 GetUpdate


blue : Color
blue =
    rgb255 100 100 255


green : Color
green =
    rgb255 75 255 75


red : Color
red =
    rgb255 255 75 75


dark : Color
dark =
    rgb255 20 20 20


grey : Color
grey =
    rgb255 130 130 130


showNamespace : Namespace -> Element Msg
showNamespace ns =
    el [ getColor ns, Font.alignLeft ] <| text ns.name


headerAttr : List (Attr decorative msg)
headerAttr =
    [ Font.size 20, Font.color blue ]


nsTable : Status -> Model -> Element Msg
nsTable status model =
    table
        [ spacing 10
        , padding 10
        , width (fill |> maximum 950)
        , Font.alignRight
        , Font.size 18
        ]
        { data = status.namespaces
        , columns =
            [ { header = el (Font.alignLeft :: headerAttr) <| text "Namespace"
              , width = fillPortion 3
              , view = showNamespace
              }
            , { header = el headerAttr <| text "Memory (Gi)"
              , width = fillPortion 1
              , view = \ns -> el [ getColor ns ] <| text <| String.fromInt ns.memUsed
              }
            , { header = el headerAttr <| text "Limit (Gi)"
              , width = fillPortion 1
              , view = showLimit model
              }
            , { header = el headerAttr <| text "Auto Start"
              , width = fillPortion 1
              , view = showStart model
              }
            , { header = el headerAttr <| text "Remaining"
              , width = fillPortion 1
              , view = showRemaining
              }
            , { header = none
              , width = shrink
              , view = extendButton
              }
            , { header = none
              , width = fillPortion 1
              , view = \_ -> none
              }
            ]
        }


startString : Maybe Int -> String
startString value =
    case value of
        Nothing ->
            "-"

        Just x ->
            String.fromInt x


showStart : Model -> Namespace -> Element Msg
showStart model ns =
    el
        [ Font.alignRight
        , Events.onMouseEnter <| EditStart <| Just ns.name
        , Events.onMouseLeave <| EditStart Nothing
        , getColor ns
        ]
    <|
        if editing model.editStart ns.name then
            startEditor ns.name ns.autoStartHour

        else
            text <| startString ns.autoStartHour


decAutostart : Maybe Int -> Maybe Int
decAutostart value =
    case value of
        Nothing ->
            Just 23

        Just x ->
            if x == 0 then
                Nothing

            else
                Just <| x - 1


incAutostart : Maybe Int -> Maybe Int
incAutostart value =
    case value of
        Nothing ->
            Just 0

        Just x ->
            if x == 23 then
                Nothing

            else
                Just <| x + 1


startEditor : String -> Maybe Int -> Element Msg
startEditor namespace value =
    let
        decButton =
            Input.button []
                { onPress = Just <| SetStart namespace <| decAutostart value
                , label = text "< "
                }

        incButton =
            Input.button []
                { onPress = Just <| SetStart namespace <| incAutostart value
                , label = text " >"
                }
    in
    row [ alignRight ]
        [ decButton
        , text (startString value)
        , incButton
        ]


editing : Maybe String -> String -> Bool
editing flag name =
    case flag of
        Just ns ->
            ns == name

        _ ->
            False


limitEditor : String -> Int -> Element Msg
limitEditor namespace value =
    let
        decButton =
            if value > 10 then
                Input.button []
                    { onPress = Just <| SetLimit namespace <| value - 10
                    , label = text "< "
                    }

            else
                el [ Font.color grey ] <| text "< "

        incButton =
            if value < 100 then
                Input.button []
                    { onPress = Just <| SetLimit namespace <| value + 10
                    , label = text " >"
                    }

            else
                el [ Font.color grey ] <| text " >"
    in
    row [ alignRight ]
        [ decButton
        , text (String.fromInt value)
        , incButton
        ]


showLimit : Model -> Namespace -> Element Msg
showLimit model ns =
    el
        [ getColor ns
        , Font.alignRight
        , Events.onMouseEnter <| EditLimit <| Just ns.name
        , Events.onMouseLeave <| EditLimit Nothing
        ]
    <|
        if editing model.editLimit ns.name then
            limitEditor ns.name ns.memLimit

        else
            text <| String.fromInt ns.memLimit


getColor : Namespace -> Attribute msg
getColor ns =
    if ns.hasDownQuota && ns.memUsed > 0 then
        Font.color red

    else if ns.memUsed > ns.memLimit then
        Font.color red

    else if not ns.hasDownQuota then
        Font.color green

    else
        Font.color grey


showRemaining : Namespace -> Element Msg
showRemaining ns =
    case ns.remaining of
        Nothing ->
            none

        Just x ->
            el [ Font.alignRight, getColor ns ] <| text x


extendButton : Namespace -> Element Msg
extendButton ns =
    if ns.canExtend then
        Input.button
            [ alignRight
            , Border.width 0
            , Border.rounded 3
            ]
            { onPress = Just <| Extend ns.name, label = text ">" }

    else
        none


defaultPage : List (Element Msg) -> Element Msg
defaultPage content =
    column
        [ Font.color grey, spacing 20, padding 20, width fill, height fill ]
        content


errorPage : String -> Element Msg
errorPage message =
    defaultPage [ title "", el [] (text message) ]


title : String -> Element Msg
title clock =
    row [ width fill ]
        [ el [ Font.size 40 ] <| text "Pod Reaper"
        , --el [ Font.size 14, Font.alignRight, width (fill |> maximum 610) ] <|
          --text clock
          row [ alignRight]
            [ Input.button[ padding 20] { label = text "Restart Podreaper", onPress = Just Restart }
            , text clock
            ]
        ]


page : Status -> Model -> Element Msg
page status model =
    defaultPage
        [ title status.clock
        , nsTable status model
        ]


defaultStyle : List (Attribute msg)
defaultStyle =
    [ Background.color dark
    , Font.color grey
    ]


nsDecoder : D.Decoder Namespace
nsDecoder =
    D.map7 Namespace
        (D.field "name" D.string)
        (D.field "hasDownQuota" D.bool)
        (D.field "canExtend" D.bool)
        (D.field "memUsed" D.int)
        (D.field "memLimit" D.int)
        (D.maybe <| D.field "autoStartHour" D.int)
        (D.maybe <| D.field "remaining" D.string)


decodeNamespaces : Maybe (List Namespace) -> D.Decoder (List Namespace)
decodeNamespaces ns =
    D.succeed (Maybe.withDefault [] ns)


statusDecoder : D.Decoder Status
statusDecoder =
    D.map2 Status
        (D.field "clock" D.string)
        (D.maybe (D.field "namespaces" <| D.list nsDecoder) |> D.andThen decodeNamespaces)


msgDecoder : String -> Result D.Error Status
msgDecoder json =
    D.decodeString statusDecoder json


view : Model -> Html.Html Msg
view model =
    case model.state of
        LoadFailed msg ->
            layout defaultStyle <| errorPage <| "Unable to retrieve namespace data: " ++ msg

        Loading ->
            layout defaultStyle <| errorPage "Loading..."

        Loaded status ->
            layout defaultStyle <| page status model
